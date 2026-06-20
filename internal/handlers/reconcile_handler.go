package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"cw7/internal/services"
	"cw7/pkg/response"

	"github.com/gin-gonic/gin"
)

type ReconcileHandler struct {
	svc services.ReconcileService
}

func NewReconcileHandler(svc services.ReconcileService) *ReconcileHandler {
	return &ReconcileHandler{svc: svc}
}

type reconcileReq struct {
	ReconcileDate string                        `json:"reconcile_date" binding:"required"`
	BankFlowBatch string                        `json:"bank_flow_batch"`
	Statements    []services.BankStatementInput `json:"statements"`
}

func (h *ReconcileHandler) Check(c *gin.Context) {
	var req reconcileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.CodeBadRequest, "参数错误: "+err.Error())
		return
	}
	ctx := c.Request.Context()
	res, err := h.svc.Check(ctx, &services.ReconcileRequest{
		ReconcileDate: req.ReconcileDate,
		BankFlowBatch: req.BankFlowBatch,
		Statements:    req.Statements,
	})
	if err != nil {
		response.Fail(c, response.CodeServerError, "对账执行失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, response.Response{
		Code:    response.CodeSuccess,
		Message: "对账完成",
		Data:    res,
	})
}

func (h *ReconcileHandler) GetResult(c *gin.Context) {
	batchNo := c.Param("batch_no")
	if batchNo == "" {
		response.Fail(c, response.CodeBadRequest, "batch_no 不能为空")
		return
	}
	res, err := h.svc.GetResult(c.Request.Context(), batchNo)
	if err != nil {
		response.Fail(c, response.CodeServerError, err.Error())
		return
	}
	if res == nil {
		response.Fail(c, response.CodeNotFound, "对账批次不存在")
		return
	}
	response.Success(c, res)
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func queryInt64(c *gin.Context, key string) (int64, error) {
	raw := c.Query(key)
	return strconv.ParseInt(raw, 10, 64)
}
