package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// serviceIDForVps 从 /vps/{name}/serviceInfos 拿 serviceId,engagement 端点需要数字 ID。
func serviceIDForVps(client *ovhsdk.Client, svc string) (int64, error) {
	var info map[string]interface{}
	if err := client.Get("/vps/"+svc+"/serviceInfos", &info); err != nil {
		return 0, err
	}
	id, _ := numconv.ToInt64(info["serviceId"])
	if id <= 0 {
		return 0, fmt.Errorf("serviceInfos 未返回 serviceId")
	}
	return id, nil
}

// GetVpsEngagement GET /api/vps-control/:service_name/engagement
func GetVpsEngagement(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		serviceID, err := serviceIDForVps(client, svc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		var eng map[string]interface{}
		if err := client.Get(fmt.Sprintf("/services/%d/billing/engagement", serviceID), &eng); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "engagement": nil, "serviceId": serviceID})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "engagement": eng, "serviceId": serviceID})
	}
}

// GetVpsEngagementAvailable GET /api/vps-control/:service_name/engagement/available
func GetVpsEngagementAvailable(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		serviceID, err := serviceIDForVps(client, svc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		var pricings []map[string]interface{}
		if err := client.Get(fmt.Sprintf("/services/%d/billing/engagement/available", serviceID), &pricings); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "pricings": pricings})
	}
}

// GetVpsEngagementRequest GET /api/vps-control/:service_name/engagement/request
func GetVpsEngagementRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		serviceID, err := serviceIDForVps(client, svc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		var req map[string]interface{}
		if err := client.Get(fmt.Sprintf("/services/%d/billing/engagement/request", serviceID), &req); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "request": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "request": req})
	}
}

// CreateVpsEngagementRequest POST /api/vps-control/:service_name/engagement/request
func CreateVpsEngagementRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			PricingMode string `json:"pricingMode"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.PricingMode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 pricingMode 参数"})
			return
		}
		serviceID, err := serviceIDForVps(client, svc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		var result map[string]interface{}
		if err := client.Post(fmt.Sprintf("/services/%d/billing/engagement/request", serviceID),
			map[string]interface{}{"pricingMode": body.PricingMode}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s engagement 请求已提交: %s", svc, body.PricingMode), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "合同期变更请求已提交", "request": result})
	}
}

// DeleteVpsEngagementRequest DELETE /api/vps-control/:service_name/engagement/request
func DeleteVpsEngagementRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		serviceID, err := serviceIDForVps(client, svc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		if err := client.Delete(fmt.Sprintf("/services/%d/billing/engagement/request", serviceID), nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s engagement 请求已撤销", svc), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "合同期变更请求已撤销"})
	}
}

// UpdateVpsEngagementEndRule PUT /api/vps-control/:service_name/engagement/end-rule
func UpdateVpsEngagementEndRule(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Strategy string `json:"strategy"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Strategy == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 strategy 参数"})
			return
		}
		serviceID, err := serviceIDForVps(client, svc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		if err := client.Put(fmt.Sprintf("/services/%d/billing/engagement/endRule", serviceID),
			map[string]interface{}{"strategy": body.Strategy}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s engagement endRule 已改为 %s", svc, body.Strategy), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "到期策略已更新"})
	}
}
