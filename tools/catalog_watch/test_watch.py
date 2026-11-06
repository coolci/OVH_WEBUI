"""Offline tests: no OVH requests, no local backend, no orders."""
import importlib.util
from contextlib import closing
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('catalog_watch', Path(__file__).with_name('watch.py'))
watch = importlib.util.module_from_spec(spec)
spec.loader.exec_module(watch)


def catalog(code='p1', addons=('ram-32',), dcs=('gra','rbx')):
    return {'plans':[{'planCode':code,'addonFamilies':[{'name':'memory','addons':list(addons),'exclusive':True,'mandatory':True}],
                      'configurations':[{'name':'dedicated_datacenter','values':list(dcs)}]}]}


class WatchTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.path = str(Path(self.temp.name)/'watch.db')
        self.identity = {'subsidiary':'IE','base':'http://localhost','accounts':[], 'execute':True}
        self.store = watch.Store(self.path, self.identity)
        self.store.seed(watch.normalize(catalog()))

    def tearDown(self):
        self.store.db.close()
        self.temp.cleanup()

    def add(self):
        payload=catalog()
        payload['plans']+=catalog('new')['plans']
        return self.store.scan(watch.normalize(payload), ['a','b'])

    def test_new_model_all_accounts_all_datacenters_unpaid(self):
        self.assertEqual(self.add(),4)
        for raw, in self.store.db.execute('SELECT payload FROM jobs'):
            body=json.loads(raw)
            self.assertIs(body['autoPay'],False)
            self.assertEqual(body['quantity'],1)
            self.assertEqual(body['planCode'],'new')
        self.assertEqual(self.add(),0)

    def test_only_new_config_no_repurchase_old_config(self):
        self.assertEqual(self.store.scan(watch.normalize(catalog(addons=('ram-32','ram-64'))), ['a']),2)
        for raw, in self.store.db.execute('SELECT payload FROM jobs'):
            self.assertEqual(json.loads(raw)['options'],['ram-64'])

    def test_price_description_ordering_do_not_trigger(self):
        payload=catalog(dcs=('rbx','gra'))
        payload['plans'][0].update(pricings=[{'price':999}],description='renamed')
        self.assertEqual(self.store.scan(watch.normalize(payload),['a']),0)

    def test_restart_dedup_and_reappearance(self):
        self.add()
        self.store.db.close()
        self.store=watch.Store(self.path,self.identity)
        self.assertEqual(self.add(),0)
        with self.assertRaises(ValueError):
            self.store.scan(watch.normalize(catalog()),['a']) # suspicious partial response
        self.assertEqual(self.add(),0)

    def test_baseline_and_state_mode_isolation(self):
        other=Path(self.temp.name)/'empty.db'
        dry=watch.Store(other,{**self.identity,'execute':False})
        try:
            self.assertEqual(dry.scan(watch.normalize(catalog()),['a']),0)
            self.assertEqual(dry.db.execute('SELECT count(*) FROM jobs').fetchone()[0],0)
        finally: dry.db.close()
        with self.assertRaises(ValueError): watch.Store(other,self.identity)

    def test_bad_catalog_and_expansion_do_not_acknowledge_changes(self):
        with self.assertRaises(ValueError): watch.normalize({'plans':[]})
        before=self.store.snapshot()
        self.assertEqual(self.store.scan(watch.normalize(catalog(addons=('a','b','c'))),['a'],limit=2),0)
        self.assertEqual(self.store.snapshot(),before)
        missing=catalog();missing['plans'][0]['configurations']=[]
        self.assertEqual(self.store.scan(watch.normalize(missing),['a']),0)
        self.assertEqual(self.store.snapshot(),before)

    def test_optional_and_exclusive_families_are_separate_orders(self):
        plan=watch.normalize(catalog(dcs=('gra',)))['p1']
        plan['families']['network']={'addons':['bw1','bw2'],'mandatory':False,'exclusive':True}
        variants=list(watch.variants(plan,100))
        self.assertEqual(len(variants),3)
        self.assertFalse(any('bw1' in options and 'bw2' in options for options,_ in variants))

    def test_account_sales_region_matching(self):
        accounts=[{'id':'a','endpoint':'ovh-eu','zone':'IE'}, {'id':'b','endpoint':'ovh-us','zone':'US'},
                  {'id':'bad','endpoint':'ovh-us','zone':'IE'}, {'id':'c','endpoint':'ovh-eu','zone':'FR'}]
        self.assertEqual(watch.accounts_for(accounts,'IE'),['a'])
        self.assertEqual(watch.accounts_for(accounts,'IE',['b']),[])

    def test_timeout_not_retried_and_queue_evidence_reconciles(self):
        self.add()
        class Fake:
            def request(self,*args,**kwargs): raise TimeoutError()
        watch.dispatch_one(self.store,Fake())
        key,raw=self.store.db.execute("SELECT id,payload FROM jobs WHERE status='uncertain'").fetchone()
        body=json.loads(raw)
        self.store.finish(key,'uncertain',delay=0)
        class Queue:
            def request(self,*args,**kwargs):
                return 200,[{**body,'accountId':body['account_id'],'id':'task1'}],{}
        watch.reconcile(self.store,Queue())
        self.assertEqual(self.store.db.execute('SELECT status FROM jobs WHERE id=?',(key,)).fetchone()[0],'accepted')

    def test_uncertain_without_evidence_never_becomes_pending(self):
        self.add(); row=self.store.claim()
        class Empty:
            def request(self,*args,**kwargs): return 200,[],{}
        watch.reconcile(self.store,Empty())
        self.assertEqual(self.store.db.execute('SELECT status FROM jobs WHERE id=?',(row[0],)).fetchone()[0],'uncertain')

    def test_submit_boundary_never_pays_and_retries_definite_rejection(self):
        self.add()
        sent=[]
        class Fake:
            def request(self,url,body,**kwargs):
                sent.append(body)
                return 400,{'error':'temporarily unavailable'},{}
        watch.dispatch_one(self.store,Fake())
        self.assertIs(sent[0]['autoPay'],False)
        self.assertEqual(sent[0]['quantity'],1)
        self.assertEqual(self.store.db.execute('SELECT status FROM jobs WHERE attempts=1').fetchone()[0],'pending')

    def test_read_only_existing_application_cache(self):
        path=Path(self.temp.name)/'app.db'
        with closing(sqlite3.connect(path)) as db:
            db.execute('CREATE TABLE catalogs(subsidiary TEXT,data TEXT,updated_at INTEGER)')
            db.execute('INSERT INTO catalogs VALUES(?,?,?)',('IE',json.dumps(catalog()),123))
            db.commit()
        self.assertEqual(watch.read_baseline(path,'IE'),self.store.snapshot())
        with closing(sqlite3.connect(path)) as db:
            self.assertEqual(db.execute('SELECT updated_at FROM catalogs').fetchone()[0],123)

    def test_instance_lock_and_application_db_protection(self):
        path=Path(self.temp.name)/'watch.lock'
        first=watch.InstanceLock(path)
        try:
            with self.assertRaises(ValueError): watch.InstanceLock(path)
        finally: first.close()
        second=watch.InstanceLock(path);second.close()
        app_path=Path(self.temp.name)/'production.db'
        with closing(sqlite3.connect(app_path)) as db:
            db.execute('CREATE TABLE catalogs(data TEXT)');db.commit()
        with self.assertRaises(ValueError): watch.Store(app_path,self.identity)
        with closing(sqlite3.connect(app_path)) as db:
            self.assertEqual(db.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall(),[('catalogs',)])

    def test_retry_after(self):
        self.assertEqual(watch.retry_after_seconds({'Retry-After':'120'}),120)
        self.assertEqual(watch.retry_after_seconds({'retry-after':'bad'}),0)

    def test_actual_http_transport_only_posts_unpaid_payload_to_backend(self):
        import http.server
        import threading
        received=[]
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                raw=self.rfile.read(int(self.headers['Content-Length']))
                received.append((self.path,dict(self.headers),json.loads(raw)))
                self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers()
                self.wfile.write(b'{"success":true}')
            def log_message(self,*args): pass
        server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler)
        thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
        self.add()
        try:
            http=watch.HTTP('http://127.0.0.1:'+str(server.server_port),'offline-test-key')
            watch.dispatch_one(self.store,http)
            path,headers,body=received[0]
            self.assertEqual(path,'/api/queue/quick-order')
            self.assertIs(body['autoPay'],False)
            self.assertEqual(body['quantity'],1)
            self.assertTrue(any(k.lower()=='x-api-key' and v=='offline-test-key' for k,v in headers.items()))
            self.assertEqual(self.store.db.execute("SELECT count(*) FROM jobs WHERE status='accepted'").fetchone()[0],1)
        finally:
            server.shutdown();server.server_close();thread.join()


    def test_optional_software_licenses_do_not_create_machine_variants(self):
        payload=catalog()
        payload['plans'][0]['addonFamilies'].append({'name':'application-license','mandatory':False,'exclusive':False,'addons':['license-'+str(i) for i in range(30)]})
        self.assertEqual(watch.normalize(payload),self.store.snapshot())



if __name__=='__main__': unittest.main()
