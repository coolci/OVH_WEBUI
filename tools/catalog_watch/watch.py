#!/usr/bin/env python3
"""Independent catalog poller + durable unpaid-order outbox. Python stdlib only."""
from __future__ import annotations

import argparse
import atexit
from contextlib import closing
from email.utils import parsedate_to_datetime
import hashlib
import itertools
import json
import logging
import os
from pathlib import Path
import sqlite3
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

REGIONS = {**dict.fromkeys('CZ DE ES EU FI FR GB IE IT LT MA NL PL PT SN TN'.split(), 'eu'),
           **dict.fromkeys('ASIA AU CA IN QC SG WE WS'.split(), 'ca'), 'US': 'us'}
HOSTS = {'eu': 'https://eu.api.ovh.com', 'ca': 'https://ca.api.ovh.com', 'us': 'https://api.us.ovhcloud.com'}
LOG = logging.getLogger('catalog-watch')
MAX_BODY = 64 * 1024 * 1024


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False)


def digest(value):
    return hashlib.sha256(encoded(value).encode()).hexdigest()


def normalize(payload):
    """Ignore prices, descriptions and array ordering. Keep ordering dimensions only."""
    plans = payload.get('plans') if isinstance(payload, dict) else None
    if not isinstance(plans, list) or not plans:
        raise ValueError('empty/malformed catalog; previous baseline retained')
    result = {}
    for plan in plans:
        code = plan.get('planCode', '')
        if not isinstance(code, str) or not code or any(c.isspace() for c in code) or code in result:
            raise ValueError('invalid or duplicate planCode')
        families = {}
        for family in plan.get('addonFamilies', []):
            name = family['name']
            if name in ('application-license', 'distribution-license'):
                if family.get('mandatory', False):
                    raise ValueError('required software licensing is unsupported for automatic orders')
                continue
            addons = family['addons']
            if name in families or not isinstance(addons, list) or any(not isinstance(a, str) or not a for a in addons):
                raise ValueError('invalid addon family')
            # Hardware families are single-choice unless explicitly marked otherwise.
            families[name] = {'addons': sorted(set(addons)),
                              'exclusive': family.get('exclusive', True),
                              'mandatory': family.get('mandatory', True)}
        if any(not isinstance(f[k], bool) for f in families.values() for k in ('exclusive', 'mandatory')):
            raise ValueError('invalid family selection flags')
        dcs = set()
        for config in plan.get('configurations', []):
            if config.get('name') == 'dedicated_datacenter':
                dcs.update(config.get('values', []))
        if any(not isinstance(dc, str) or not dc for dc in dcs):
            raise ValueError('invalid datacenter')
        result[code] = {'families': families, 'datacenters': sorted(dcs)}
    return result


def choices(family):
    addons = family['addons']
    if family['exclusive']:
        return ([()] if not family['mandatory'] else []) + [(a,) for a in addons]
    start = 1 if family['mandatory'] else 0
    return [c for n in range(start, len(addons) + 1) for c in itertools.combinations(addons, n)]


def variants(plan, limit):
    """One selection per exclusive family, every subset for non-exclusive families."""
    groups = []
    count = max(1, len(plan['datacenters']))
    for name, family in sorted(plan['families'].items()):
        n = len(family['addons'])
        if not family['exclusive'] and n > limit.bit_length():
            raise ValueError('non-exclusive configuration expansion exceeds limit')
        width = n + (not family['mandatory']) if family['exclusive'] else 2 ** n - bool(family['mandatory'])
        count *= width
        if count > limit:
            raise ValueError(f'configuration expansion exceeds --max-combinations={limit}; plan deferred intact')
        groups.append(choices(family))
    if not plan['datacenters']:
        raise ValueError('catalog has no dedicated_datacenter; cannot invent ordering locations')
    for selection in itertools.product(*groups):
        options = sorted(set(itertools.chain.from_iterable(selection)))
        for dc in plan['datacenters']:
            yield options, dc


def accounts_for(rows, subsidiary, selected=()):
    result = []
    for account in rows:
        endpoint = account.get('endpoint', 'ovh-eu')
        region = endpoint.rsplit('-', 1)[-1]
        zone = (account.get('zone') or {'eu': 'IE', 'ca': 'CA', 'us': 'US'}.get(region, '')).upper()
        if zone == subsidiary and REGIONS.get(zone) == region and (not selected or account['id'] in selected):
            result.append(account['id'])
    return sorted(set(result))


class InstanceLock:
    """OS releases this lock after a crash; prevents two scanners sharing one state."""
    def __init__(self, path):
        self.file = open(path, 'a+b')
        self.file.seek(0, 2)
        if not self.file.tell():
            self.file.write(b'0')
            self.file.flush()
        self.file.seek(0)
        try:
            if os.name == 'nt':
                import msvcrt
                msvcrt.locking(self.file.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(self.file.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            self.file.close()
            raise ValueError('another catalog watcher already owns this state')

    def close(self):
        if not self.file.closed:
            self.file.close()


class Store:
    def __init__(self, path, identity):
        self.db = sqlite3.connect(path, timeout=30)
        tables = {r[0] for r in self.db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")}
        if tables and tables != {'meta', 'plans', 'jobs'}:
            self.db.close()
            raise ValueError('--state is not a catalog-watch database; refusing to modify it')
        self.db.execute('PRAGMA journal_mode=WAL')
        self.db.executescript('''
          CREATE TABLE IF NOT EXISTS meta(k TEXT PRIMARY KEY, value TEXT NOT NULL);
          CREATE TABLE IF NOT EXISTS plans(code TEXT PRIMARY KEY, spec TEXT NOT NULL);
          CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY, payload TEXT NOT NULL, status TEXT NOT NULL,
             attempts INTEGER NOT NULL DEFAULT 0, due REAL NOT NULL DEFAULT 0, detail TEXT NOT NULL DEFAULT '');
        ''')
        row = self.db.execute("SELECT value FROM meta WHERE k='identity'").fetchone()
        if row and row[0] != encoded(identity):
            self.db.close()
            raise ValueError('state belongs to another subsidiary/backend/account selection/mode; use a separate --state')
        with self.db:
            self.db.execute("INSERT OR IGNORE INTO meta VALUES('identity', ?)", (encoded(identity),))
        self.identity = identity

    def snapshot(self):
        return {code: json.loads(spec) for code, spec in self.db.execute('SELECT code,spec FROM plans')}

    def seed(self, snapshot):
        if self.snapshot():
            return
        with self.db:
            self.db.executemany('INSERT INTO plans VALUES(?, ?)', [(c, encoded(p)) for c, p in snapshot.items()])

    def scan(self, current, accounts, limit=10000):
        old = self.snapshot()
        if not old:
            self.seed(current)
            return 0
        if len(current) < len(old) * 0.8:
            raise ValueError('catalog shrank by >20%; possible incomplete response; baseline retained')
        inserted = 0
        # Never delete old plans: disappear/reappear is not a new model.
        for code, spec in current.items():
            if old.get(code) == spec:
                continue
            try:
                previous = {(tuple(o), dc) for o, dc in variants(old[code], limit)} if code in old else set()
                additions = [(o, dc) for o, dc in variants(spec, limit) if (tuple(o), dc) not in previous]
            except ValueError as error:
                LOG.error('%s: %s', code, error)
                continue  # Do not acknowledge this change; retry on a subsequent poll.
            if additions and not accounts:
                LOG.warning('%s: no matching selected account; change remains pending', code)
                continue
            with self.db:
                for options, dc in additions:
                    for account in accounts:
                        payload = {'account_id': account, 'planCode': code, 'datacenter': dc,
                                   'options': options, 'quantity': 1, 'autoPay': False, 'force': True}
                        key = digest([self.identity['subsidiary'], account, code, dc, options])
                        cursor = self.db.execute('INSERT OR IGNORE INTO jobs(id,payload,status) VALUES(?,?,?)',
                                                 (key, encoded(payload), 'pending' if self.identity['execute'] else 'planned'))
                        inserted += cursor.rowcount
                self.db.execute('INSERT OR REPLACE INTO plans VALUES(?,?)', (code, encoded(spec)))
        return inserted

    def claim(self):
        with self.db:
            self.db.execute('BEGIN IMMEDIATE')
            row = self.db.execute("SELECT id,payload,attempts FROM jobs WHERE status='pending' AND due<=? ORDER BY rowid LIMIT 1", (time.time(),)).fetchone()
            if row:
                self.db.execute("UPDATE jobs SET status='sending',attempts=attempts+1 WHERE id=?", (row[0],))
        return row

    def finish(self, key, status, detail='', delay=0):
        with self.db:
            self.db.execute('UPDATE jobs SET status=?,detail=?,due=? WHERE id=?', (status, detail[:2000], time.time()+delay, key))


class HTTP:
    def __init__(self, base, secret):
        self.base, self.secret = base.rstrip('/'), secret

    def request(self, url, payload=None, local=False, headers=None):
        h = {'Accept': 'application/json', **(headers or {})}
        if local:
            h.update({'X-API-Key': self.secret, 'X-Request-Time': str(int(time.time()*1000))})
        data = None if payload is None else encoded(payload).encode()
        if data is not None:
            h['Content-Type'] = 'application/json'
        req = urllib.request.Request(self.base+url if local else url, data=data, headers=h)
        try:
            with urllib.request.urlopen(req, timeout=60 if local else 20) as response:
                raw = response.read(MAX_BODY + 1)
                if len(raw) > MAX_BODY:
                    raise ValueError('response exceeds size limit')
                return response.status, json.loads(raw), dict(response.headers)
        except urllib.error.HTTPError as error:
            if error.code == 304:
                return 304, None, dict(error.headers)
            try:
                body = json.loads(error.read(1024 * 1024))
            except (ValueError, OSError):
                body = {}
            return error.code, body, dict(error.headers)

    def accounts(self):
        status, body, _ = self.request('/api/accounts', local=True)
        if status != 200 or not isinstance(body, dict) or not isinstance(body.get('accounts'), list):
            raise ValueError('cannot load account list')
        return body['accounts']


def same_order(item, payload):
    return (item.get('accountId', item.get('account_id')) == payload['account_id']
            and item.get('planCode') == payload['planCode']
            and item.get('datacenter') == payload['datacenter']
            and sorted(item.get('options') or []) == payload['options'])


def dispatch_one(store, http):
    row = store.claim()
    if not row:
        return False
    key, raw, attempt = row
    payload = json.loads(raw)
    # Enforced again at the final network boundary. Configuration cannot enable payment.
    payload.update(autoPay=False, quantity=1)
    try:
        status, body, headers = http.request('/api/queue/quick-order', payload, local=True)
    except Exception as error:
        store.finish(key, 'uncertain', type(error).__name__, 60)
        return True
    if status == 200 and body.get('success') is True:
        store.finish(key, 'accepted', 'queued; not proof of a completed OVH order')
    elif status >= 500 or status == 200:
        # The backend may have committed before returning an unexpected response.
        store.finish(key, 'uncertain', 'ambiguous backend response', 60)
    elif body.get('code') in ('EE77AF09E', 'EF4FA206C'):
        store.finish(key, 'existing', body.get('error', 'matching order already exists'))
    elif status in (401, 403, 404):
        store.finish(key, 'blocked', f'backend HTTP {status}')
    else:
        # Validation rejection did not enqueue; retry stock/config propagation failures.
        store.finish(key, 'pending', body.get('error', f'HTTP {status}'), max(retry_after_seconds(headers), min(300, 5 * 2 ** min(attempt, 6))))
    LOG.info('submission %s %s %s', payload['account_id'], payload['planCode'], store.db.execute('SELECT status FROM jobs WHERE id=?',(key,)).fetchone()[0])
    return True


def reconcile(store, http):
    rows = store.db.execute("SELECT id,payload FROM jobs WHERE status IN ('sending','uncertain') AND due<=?", (time.time(),)).fetchall()
    if not rows:
        return
    status, items, _ = http.request('/api/queue', local=True)
    if status != 200 or not isinstance(items, list):
        return
    for key, raw in rows:
        matched = next((item for item in items if same_order(item, json.loads(raw))), None)
        if matched:
            store.finish(key, 'accepted', f"reconciled queue task {matched.get('id', '')}")
        else:
            # No match is not proof that no order was created (queue may have been cleared).
            store.finish(key, 'uncertain', 'no queue evidence; do not resubmit automatically', 60)


def read_baseline(path, subsidiary):
    with closing(sqlite3.connect(Path(path).resolve().as_uri()+'?mode=ro', uri=True)) as db:
        row = db.execute('SELECT data FROM catalogs WHERE subsidiary=?', (subsidiary,)).fetchone()
    if not row:
        raise ValueError('no cached catalog for this subsidiary in baseline database')
    return normalize(json.loads(row[0]))


def retry_after_seconds(headers):
    raw = next((v for k,v in headers.items() if k.lower() == 'retry-after'), '')
    try:
        return max(0, int(raw))
    except (ValueError, TypeError):
        try:
            return max(0, parsedate_to_datetime(raw).timestamp()-time.time())
        except (ValueError, TypeError, AttributeError, OverflowError):
            return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--subsidiary', required=True, choices=sorted(REGIONS))
    parser.add_argument('--base', default='http://127.0.0.1:19998')
    parser.add_argument('--state', required=True, help='independent SQLite file, never the application DB')
    parser.add_argument('--baseline-db', help='read existing application catalog cache once, read-only')
    parser.add_argument('--account', action='append', default=[], help='repeat; omitted means all matching accounts')
    parser.add_argument('--interval', type=float, default=2, help='seconds between poll starts, minimum 1')
    parser.add_argument('--max-combinations', type=int, default=10000, help='per-plan expansion limit; excess is deferred, never truncated')
    parser.add_argument('--execute', action='store_true', help='submit unpaid order tasks; otherwise only persist planned jobs')
    parser.add_argument('--once', action='store_true', help='one detection cycle, no submission worker')
    args = parser.parse_args()
    if not 1 <= args.interval <= 3600 or args.max_combinations < 1:
        parser.error('interval must be >=1 and max-combinations must be positive')
    if args.execute and args.once:
        parser.error('--once is for detection only; omit --execute')
    if args.baseline_db and Path(args.state).resolve() == Path(args.baseline_db).resolve():
        parser.error('--state must not be the application database')
    if not os.environ.get('API_SECRET_KEY'):
        parser.error('set API_SECRET_KEY in the environment')
    logging.basicConfig(level=logging.INFO, format='%(asctime)s %(levelname)s %(message)s')
    identity = {'subsidiary': args.subsidiary, 'base': args.base.rstrip('/'), 'accounts': sorted(args.account), 'execute': args.execute}
    lock = InstanceLock(args.state + '.lock')
    atexit.register(lock.close)
    store = Store(args.state, identity)
    if not store.snapshot() and args.baseline_db:
        store.seed(read_baseline(args.baseline_db, args.subsidiary))
    http = HTTP(args.base, os.environ['API_SECRET_KEY'])
    accounts = accounts_for(http.accounts(), args.subsidiary, args.account)
    if not accounts:
        parser.error('no matching accounts for the selected sales subsidiary')
    stop = threading.Event()

    def submitter():
        worker = Store(args.state, identity)
        try:
            while not stop.is_set():
                try:
                    reconcile(worker, http)
                    if not dispatch_one(worker, http):
                        stop.wait(1)
                except Exception as error:
                    LOG.error('submission worker: %s', type(error).__name__)
                    stop.wait(5)
        finally:
            worker.db.close()

    thread = threading.Thread(target=submitter, daemon=True)
    if args.execute:
        thread.start()
    url = HOSTS[REGIONS[args.subsidiary]]+'/v1/order/catalog/public/eco?'+urllib.parse.urlencode({'ovhSubsidiary': args.subsidiary})
    validators, failures, account_time = {}, 0, time.monotonic()
    try:
        while not stop.is_set():
            started = time.monotonic()
            retry_after = 0
            try:
                if started-account_time >= 60:
                    accounts = accounts_for(http.accounts(), args.subsidiary, args.account)
                    account_time = started
                retry_after = 0
                status, body, headers = http.request(url, headers={'Cache-Control': 'no-cache', **validators})
                retry_after = retry_after_seconds(headers)
                if status == 200:
                    current = normalize(body)
                    count = store.scan(current, accounts, args.max_combinations)
                    LOG.info('catalog checked: plans=%d new_jobs=%d mode=%s', len(current), count, 'execute-unpaid' if args.execute else 'dry-run')
                    # Do not use 304 if any changed plan remains deferred.
                    validators = {}
                    snapshot = store.snapshot()
                    if all(snapshot.get(c) == p for c,p in current.items()):
                        lower = {k.lower(): v for k,v in headers.items()}
                        if lower.get('etag'): validators['If-None-Match'] = lower['etag']
                        elif lower.get('last-modified'): validators['If-Modified-Since'] = lower['last-modified']
                elif status != 304:
                    raise ValueError(f'upstream HTTP {status}')
                failures = 0
            except Exception as error:
                failures += 1
                LOG.error('catalog poll: %s', error)
            if args.once:
                return 1 if failures else 0
            delay = max(args.interval, min(60, 2 ** min(failures, 6))) if failures else args.interval
            stop.wait(max(retry_after if failures else 0, delay-(time.monotonic()-started), 0))
    except KeyboardInterrupt:
        LOG.info('stopping detector; already accepted backend tasks remain in the application queue')
    finally:
        stop.set()
        if thread.is_alive(): thread.join(timeout=65)
        store.db.close()
        lock.close()
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
