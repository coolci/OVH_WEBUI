package router

import (
 "encoding/json"
 "os"
 "testing"
 "github.com/ovh-webui/server/internal/app"
 "github.com/ovh-webui/server/internal/config"
 "github.com/ovh-webui/server/internal/db"
)

// Construct the real Gin registry without starting a listener or sending requests.
func TestAuditRouteInventory(t *testing.T) {
 database, err := db.Open(t.TempDir()); if err != nil {t.Fatal(err)}; defer database.Close()
 state := &app.State{DB:database, Config:config.New(database)}
 engine := New(Config{State:state,EnableAuth:true},nil)
 registered:=engine.Routes()
 routes:=[]map[string]string{}
 for _,route:=range registered {routes=append(routes,map[string]string{"method":route.Method,"path":route.Path,"handler":route.Handler})}
 data,err:=json.MarshalIndent(routes,"","  ");if err!=nil{t.Fatal(err)}
 dest:=os.Getenv("AUDIT_ROUTE_OUTPUT");if dest==""{t.Fatal("AUDIT_ROUTE_OUTPUT required")}
 if err:=os.WriteFile(dest,data,0600);err!=nil{t.Fatal(err)}
 t.Logf("registered routes=%d; no listener or external requests",len(routes))
}
