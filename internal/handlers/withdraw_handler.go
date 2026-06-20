package handlers

import (
	"net/http"

	"cw7/internal/services"
	"cw7/pkg/response"

	"github.com/gin-gonic/gin"
)

type WithdrawHandler struct {
	svc services.WithdrawService
}

func NewWithdrawHandler(svc services.WithdrawService) *WithdrawHandler {
	return &WithdrawHandler{svc: svc}
}

type applyReq struct {
	DriverID int64   `json:"driver_id" binding:"required,min=1"`
	Amount   float64 `json:"amount" binding:"required"`
}

func (h *WithdrawHandler) Apply(c *gin.Context) {
	var req applyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.CodeBadRequest, "参数错误: "+err.Error())
		return
	}
	ctx := c.Request.Context()
	res, err := h.svc.Apply(ctx, &services.ApplyRequest{
		DriverID: req.DriverID,
		Amount:   req.Amount,
	})
	if err != nil {
		msg := err.Error()
		code := response.CodeServerError
		switch {
		case contains(msg, "less than min"), contains(msg, "exceed max"), contains(msg, "must be positive"), contains(msg, "invalid amount"):
			code = response.CodeInvalidAmount
		case contains(msg, "insufficient balance"):
			code = response.CodeBalance
		case contains(msg, "daily withdraw limit"):
			code = response.CodeWithdrawLimit
		case contains(msg, "not found"), contains(msg, "not bind"):
			code = response.CodeNotFound
		case contains(msg, "circuit breaker is open"), contains(msg, "提现通道临时关闭"):
			code = response.CodeCircuitOpen
		}
		response.Fail(c, code, msg)
		return
	}
	c.JSON(http.StatusOK, response.Response{
		Code:    response.CodeSuccess,
		Message: "提现申请提交成功",
		Data:    res,
	})
}

func (h *WithdrawHandler) GetByNo(c *gin.Context) {
	no := c.Param("no")
	if no == "" {
		response.Fail(c, response.CodeBadRequest, "提现单号不能为空")
		return
	}
	w, err := h.svc.GetByNo(c.Request.Context(), no)
	if err != nil {
		response.Fail(c, response.CodeServerError, err.Error())
		return
	}
	if w == nil {
		response.Fail(c, response.CodeNotFound, "提现记录不存在")
		return
	}
	response.Success(c, w)
}

func (h *WithdrawHandler) ListByDriver(c *gin.Context) {
	driverID, err := queryInt64(c, "driver_id")
	if err != nil || driverID <= 0 {
		response.Fail(c, response.CodeBadRequest, "driver_id 参数无效")
		return
	}
	date := c.Query("date")
	list, err := h.svc.ListByDriver(c.Request.Context(), driverID, date)
	if err != nil {
		response.Fail(c, response.CodeServerError, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *WithdrawHandler) ListCachedToday(c *gin.Context) {
	date := c.Query("date")
	list, err := h.svc.ListCachedToday(c.Request.Context(), date)
	if err != nil {
		response.Fail(c, response.CodeServerError, err.Error())
		return
	}
	response.Success(c, list)
}
