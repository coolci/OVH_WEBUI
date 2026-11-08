package ovh

import "sync"

// CartMarker 本程序建的每一辆 OVH 购物车都带这个 description。
// 清理遗留购物车时只认这个标记:用户在 OVH 网站上自己建的车、
// 以及加标记之前本程序建的老车,一律不动。
const CartMarker = "ovh-webui"

// CartCreateBody POST /order/cart 的请求体(带标记)
func CartCreateBody(subsidiary string) map[string]interface{} {
	return map[string]interface{}{"ovhSubsidiary": subsidiary, "description": CartMarker}
}

// 本进程正在使用或必须保留的购物车(下单中、预热池里、结账结果待核对)。
// 清理只删"带标记、但不在这里"的车 —— 也就是上一次运行留下的遗留车。
var inUseCarts sync.Map

// TrackCart 登记一辆本进程在用/需保留的购物车
func TrackCart(id string) {
	if id != "" {
		inUseCarts.Store(id, struct{}{})
	}
}

// CartInUse 这辆车是否被本进程登记为在用
func CartInUse(id string) bool {
	_, ok := inUseCarts.Load(id)
	return ok
}
