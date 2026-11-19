import React from "react";
import { ServerScopeContext } from "./server-scope";

export function ServerScopeProvider({
  accountId,
  children,
}: {
  accountId?: string;
  children: React.ReactNode;
}) {
  return (
    <ServerScopeContext.Provider value={accountId || undefined}>
      {children}
    </ServerScopeContext.Provider>
  );
}
