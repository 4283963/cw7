package handlers

import (
	"net/http"

	"cw7/internal/services"
	"cw7/pkg/response"

	"github.com/gin-gonic/gin"
)

type CircuitHandler struct {
	svc services.CircuitBreakerService
}

func NewCircuitHandler(svc services.CircuitBreakerService) *CircuitHandler {
	return &CircuitHandler{svc: svc}
}

type toggleReq struct {
	Operator string `json:"operator" binding:"required,min=1,max=64"`
	Reason   string `json:"reason" binding:"required,min=1,max=200"`
	Open     bool   `json:"open"`
}

func (h *CircuitHandler) Toggle(c *gin.Context) {
	var req toggleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.CodeBadRequest, "参数错误: "+err.Error())
		return
	}
	ctx := c.Request.Context()
	res, err := h.svc.Toggle(ctx, &services.CircuitToggleRequest{
		Operator: req.Operator,
		Reason:   req.Reason,
		Open:     req.Open,
	})
	if err != nil {
		response.Fail(c, response.CodeServerError, "切换提现通道状态失败: "+err.Error())
		return
	}
	var msg string
	if req.Open {
		msg = "提现通道已紧急关闭"
	} else {
		msg = "提现通道已恢复"
	}
	c.JSON(http.StatusOK, response.Response{
		Code:    response.CodeSuccess,
		Message: msg,
		Data:    res,
	})
}

func (h *CircuitHandler) Status(c *gin.Context) {
	ctx := c.Request.Context()
	res, err := h.svc.GetStatus(ctx)
	if err != nil {
		response.Fail(c, response.CodeServerError, "查询通道状态失败: "+err.Error())
		return
	}
	response.Success(c, res)
}
