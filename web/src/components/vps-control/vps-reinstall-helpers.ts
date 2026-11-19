export function resolveVpsReinstallSshKey(
  selectedSshKey: string,
  customSshKey: string,
): {
  key: string;
  valid: boolean;
  error?: "raw_pubkey" | "multiple_keys";
} {
  const raw =
    selectedSshKey === "__custom__"
      ? (customSshKey || "").trim()
      : selectedSshKey === "__none__"
        ? ""
        : (selectedSshKey || "").trim();

  if (!raw) {
    return { key: "", valid: false };
  }
  if (/^(ssh-(rsa|ed25519|dss)|ecdsa-sha2-|sk-(ssh|ecdsa)-)/i.test(raw)) {
    return { key: raw, valid: false, error: "raw_pubkey" };
  }
  if (raw.includes(",")) {
    return { key: raw, valid: false, error: "multiple_keys" };
  }
  return { key: raw, valid: true };
}

export function canEnableDoNotSendPassword(selectedSshKey: string, customSshKey: string): boolean {
  return resolveVpsReinstallSshKey(selectedSshKey, customSshKey).valid;
}
