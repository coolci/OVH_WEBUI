import { describe, expect, it } from "vitest";
import { monitorDcAvailability } from "./datacenters";
import {
  accountsWhenEnablingAutoOrder,
  customPlanCode,
  isSinglePlanCode,
  monitorAccountIds,
  parseOptionCodes,
} from "./order-inputs";

describe("order-inputs", () => {
  it("multiple order accounts survive normalization without duplicates", () => {
    expect(monitorAccountIds({ autoOrderAccountIds: [" a ", "b", "a", ""] })).toEqual(["a", "b"]);
  });

  it("legacy single-account subscriptions stay on their original account", () => {
    expect(monitorAccountIds({ autoOrderAccountId: "a" })).toEqual(["a"]);
    expect(monitorAccountIds({ autoOrderAccountIds: null, autoOrderAccountId: "a" })).toEqual(["a"]);
  });

  it("an explicit cleared list never restores the old account", () => {
    expect(monitorAccountIds({ autoOrderAccountIds: [], autoOrderAccountId: "a" })).toEqual([]);
    expect(monitorAccountIds({})).toEqual([]);
  });

  it("explicitly enabling automatic orders defaults an empty selection to all existing accounts", () => {
    expect(accountsWhenEnablingAutoOrder(false, true, [], ["a", "b"])).toEqual(["a", "b"]);
  });

  it("opening, saving, clearing, and existing selections never silently expand account scope", () => {
    expect(accountsWhenEnablingAutoOrder(false, false, [], ["a", "b"])).toEqual([]);
    expect(accountsWhenEnablingAutoOrder(true, true, [], ["a", "b"])).toEqual([]);
    expect(accountsWhenEnablingAutoOrder(false, true, ["b"], ["a", "b"])).toEqual(["b"]);
  });

  it("custom models are available with an empty or incomplete catalog", () => {
    expect(customPlanCode("  26custom-v1  ", [])).toBe("26custom-v1");
    expect(customPlanCode("vps-custom-eu", [{ planCode: "vps-known" }])).toBe("vps-custom-eu");
  });

  it("a catalog match uses its existing option regardless of case", () => {
    expect(customPlanCode("24SK102", [{ planCode: "24sk102" }])).toBe("");
  });

  it("multiple codes and descriptive search text cannot become an accidental order code", () => {
    for (const value of [
      "",
      " ",
      "plan-a,plan-b",
      "plan-a，plan-b",
      "plan-a、plan-b",
      "Intel Xeon",
      "plan-a\nplan-b",
    ]) {
      expect(isSinglePlanCode(value), value).toBe(false);
      expect(customPlanCode(value, []), value).toBe("");
    }
  });

  it("manually entered hardware addons preserve arbitrary codes and deduplicate separators", () => {
    expect(parseOptionCodes(" ram-custom，storage-custom, ram-custom\n network-custom ")).toEqual([
      "ram-custom",
      "storage-custom",
      "network-custom",
    ]);
  });
});

describe("monitorDcAvailability", () => {
  it("combines per-account and per-configuration stock without hiding stock", () => {
    expect(
      monitorDcAvailability({
        'account:"a"|gra|ram-64': "1H-low",
        'account:"b"|gra|ram-64': "unavailable",
        'account:"a"|rbx|ram-64': "unavailable",
        'account:"b"|rbx|ram-128': "72H",
        'account:"a"|ynm|ram-64': "unavailable",
      })
    ).toEqual({ gra: "1H-low", rbx: "72H", mum: "unavailable" });
  });

  it("reads legacy keys and JSON-encoded account IDs", () => {
    expect(
      monitorDcAvailability({
        gra: "unknown",
        "gra|memory": "1H-low",
        [`account:${JSON.stringify('id|with"quotes')}|sbg|memory`]: "unavailable",
        "account:malformed|vin": "1H-low",
      })
    ).toEqual({ gra: "1H-low", sbg: "unavailable" });
  });

  it("ignores saved stock baselines for deselected accounts", () => {
    expect(
      monitorDcAvailability(
        {
          "gra|memory": "available",
          'account:"a"|gra|memory': "unavailable",
          'account:"b"|gra|memory': "1H-low",
          'account:"b"|rbx|memory': "1H-low",
        },
        ["a"]
      )
    ).toEqual({ gra: "unavailable" });
  });

  it("notification-only subscriptions still show legacy inventory without showing removed accounts", () => {
    expect(
      monitorDcAvailability(
        {
          "gra|memory": "available",
          'account:"a"|rbx|memory': "available",
        },
        []
      )
    ).toEqual({ gra: "available" });
  });
});
