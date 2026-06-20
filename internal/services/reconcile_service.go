package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"cw7/internal/models"
	"cw7/internal/repositories"
	"cw7/pkg/snowflake"
)

type BankStatementInput struct {
	BankRefNo   string  `json:"bank_ref_no" binding:"required"`
	TrxDate     string  `json:"trx_date" binding:"required"`
	TrxTime     string  `json:"trx_time" binding:"required"`
	FlowType    int     `json:"flow_type" binding:"required,oneof=1 2"`
	Amount      float64 `json:"amount" binding:"required"`
	Balance     float64 `json:"balance"`
	OppBankCard string  `json:"opp_bank_card" binding:"required"`
	OppName     string  `json:"opp_name"`
	Summary     string  `json:"summary"`
}

type ReconcileRequest struct {
	ReconcileDate string               `json:"reconcile_date" binding:"required"`
	BankFlowBatch string               `json:"bank_flow_batch"`
	Statements    []BankStatementInput `json:"statements"`
}

type ReconcileResult struct {
	BatchNo         string             `json:"batch_no"`
	ReconcileDate   string             `json:"reconcile_date"`
	TotalSystem     int                `json:"total_system"`
	TotalBank       int                `json:"total_bank"`
	MatchedCount    int                `json:"matched_count"`
	SystemTotalAmt  float64            `json:"system_total_amt"`
	BankTotalAmt    float64            `json:"bank_total_amt"`
	MatchedTotalAmt float64            `json:"matched_total_amt"`
	DiffCount       int                `json:"diff_count"`
	DiffList        []ReconcileDiffOut `json:"diff_list,omitempty"`
	Status          int                `json:"status"`
	CreatedAt       string             `json:"created_at"`
}

type ReconcileDiffOut struct {
	DiffType     string  `json:"diff_type"`
	WithdrawNo   string  `json:"withdraw_no,omitempty"`
	BankRefNo    string  `json:"bank_ref_no,omitempty"`
	SystemAmount float64 `json:"system_amount,omitempty"`
	BankAmount   float64 `json:"bank_amount,omitempty"`
	DiffAmount   float64 `json:"diff_amount,omitempty"`
	SystemCard   string  `json:"system_card,omitempty"`
	BankCard     string  `json:"bank_card,omitempty"`
	Remark       string  `json:"remark"`
}

const amountEpsilon = 0.005

type ReconcileService interface {
	Check(ctx context.Context, req *ReconcileRequest) (*ReconcileResult, error)
	GetResult(ctx context.Context, batchNo string) (*ReconcileResult, error)
}

type reconcileService struct {
	withdrawRepo  repositories.WithdrawalRepository
	bankRepo      repositories.BankStatementRepository
	reconcileRepo repositories.ReconcileRepository
	idGen         *snowflake.Snowflake
}

func NewReconcileService(
	withdrawRepo repositories.WithdrawalRepository,
	bankRepo repositories.BankStatementRepository,
	reconcileRepo repositories.ReconcileRepository,
	idGen *snowflake.Snowflake,
) ReconcileService {
	return &reconcileService{
		withdrawRepo:  withdrawRepo,
		bankRepo:      bankRepo,
		reconcileRepo: reconcileRepo,
		idGen:         idGen,
	}
}

func genBatchNo(date string, id int64) string {
	return fmt.Sprintf("RC%s%014d", dateFormat(date), id%100000000000000)
}

func dateFormat(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return t.Format("20060102")
}

func toReconcileDiffOut(d models.ReconcileDiff) ReconcileDiffOut {
	out := ReconcileDiffOut{
		DiffType: d.DiffType,
		Remark:   d.Remark,
	}
	if d.WithdrawNo != nil {
		out.WithdrawNo = *d.WithdrawNo
	}
	if d.BankRefNo != nil {
		out.BankRefNo = *d.BankRefNo
	}
	if d.SystemAmount != nil {
		out.SystemAmount = *d.SystemAmount
	}
	if d.BankAmount != nil {
		out.BankAmount = *d.BankAmount
	}
	if d.DiffAmount != nil {
		out.DiffAmount = *d.DiffAmount
	}
	if d.SystemCard != nil {
		out.SystemCard = *d.SystemCard
	}
	if d.BankCard != nil {
		out.BankCard = *d.BankCard
	}
	return out
}

func (s *reconcileService) Check(ctx context.Context, req *ReconcileRequest) (*ReconcileResult, error) {
	if req == nil {
		return nil, errors.New("empty request")
	}
	if req.ReconcileDate == "" {
		return nil, errors.New("reconcile_date is required")
	}
	date := req.ReconcileDate
	batchNo := genBatchNo(date, s.idGen.NextID())

	systemWithdrawals, err := s.withdrawRepo.ListPendingByDate(ctx, date)
	if err != nil {
		return nil, fmt.Errorf("list system withdrawals error: %w", err)
	}

	var bankStmts []models.BankStatement
	if len(req.Statements) > 0 {
		bankStmts, err = s.buildBankStatements(ctx, req.Statements, batchNo)
		if err != nil {
			return nil, err
		}
	} else {
		bankStmts, err = s.bankRepo.ListByDate(ctx, date)
		if err != nil {
			return nil, fmt.Errorf("list bank statements error: %w", err)
		}
	}

	debitStmts := filterDebitOnly(bankStmts)

	sysTotal, bankTotal := sumAmounts(systemWithdrawals, debitStmts)

	// 1) 先按银行卡+金额精确匹配
	matchedBank := make(map[int64]struct{})
	matchedSys := make(map[int64]struct{})
	matchedMap := make(map[int64]int64) // bankStmtID -> withdrawID
	matchedAmt := 0.0
	matchedCount := 0

	cardAmtIndex := buildCardAmtIndex(debitStmts, matchedBank)
	for i := range systemWithdrawals {
		w := systemWithdrawals[i]
		if _, ok := matchedSys[w.ID]; ok {
			continue
		}
		key := indexKey(w.BankCard, w.ActualAmount)
		if list, ok := cardAmtIndex[key]; ok {
			for j := range list {
				bs := &debitStmts[list[j]]
				if _, used := matchedBank[bs.ID]; used {
					continue
				}
				matchedBank[bs.ID] = struct{}{}
				matchedSys[w.ID] = struct{}{}
				matchedMap[bs.ID] = w.ID
				matchedAmt += w.ActualAmount
				matchedCount++
				break
			}
		}
	}

	// 2) 剩余按金额（允许±0.01内误差）+银行卡模糊匹配
	remainingSys := filterUnmatched(systemWithdrawals, matchedSys)
	remainingBank := filterBankUnmatched(debitStmts, matchedBank)
	cardIndex := buildCardIndex(remainingBank)

	for i := range remainingSys {
		w := remainingSys[i]
		list, ok := cardIndex[w.BankCard]
		if !ok {
			continue
		}
		for j := range list {
			bs := list[j]
			if _, used := matchedBank[bs.ID]; used {
				continue
			}
			if math.Abs(bs.Amount-w.ActualAmount) <= amountEpsilon {
				matchedBank[bs.ID] = struct{}{}
				matchedSys[w.ID] = struct{}{}
				matchedMap[bs.ID] = w.ID
				matchedAmt += w.ActualAmount
				matchedCount++
				break
			}
		}
	}

	// 3) 再次剩余的仅按金额匹配（兜底）
	remainingSys = filterUnmatched(systemWithdrawals, matchedSys)
	remainingBank = filterBankUnmatched(debitStmts, matchedBank)
	amtIndex := buildAmtIndex(remainingBank)

	for i := range remainingSys {
		w := remainingSys[i]
		key := amtKey(w.ActualAmount)
		list, ok := amtIndex[key]
		if !ok {
			continue
		}
		for j := range list {
			bs := list[j]
			if _, used := matchedBank[bs.ID]; used {
				continue
			}
			matchedBank[bs.ID] = struct{}{}
			matchedSys[w.ID] = struct{}{}
			matchedMap[bs.ID] = w.ID
			matchedAmt += w.ActualAmount
			matchedCount++
			break
		}
	}

	diffs := s.buildDiffs(systemWithdrawals, debitStmts, matchedSys, matchedBank, matchedMap, batchNo, wl(debitStmts))

	matchedIDs := make([]int64, 0, len(matchedSys))
	for id := range matchedSys {
		matchedIDs = append(matchedIDs, id)
	}

	if len(req.Statements) > 0 {
		if err := s.bankRepo.BatchInsert(ctx, bankStmts); err != nil {
			return nil, fmt.Errorf("insert bank statements error: %w", err)
		}
	}

	now := time.Now()
	log := &models.ReconcileLog{
		ID:              s.idGen.NextID(),
		BatchNo:         batchNo,
		ReconcileDate:   date,
		TotalSystem:     len(systemWithdrawals),
		TotalBank:       len(debitStmts),
		MatchedCount:    0,
		SystemTotalAmt:  sysTotal,
		BankTotalAmt:    bankTotal,
		MatchedTotalAmt: 0,
		DiffCount:       0,
		Status:          1,
		CreatedAt:       now,
	}
	if err := s.reconcileRepo.CreateLog(ctx, log); err != nil {
		return nil, fmt.Errorf("create reconcile log error: %w", err)
	}

	if err := s.bankRepo.UpdateMatchedBatch(ctx, matchedMap); err != nil {
		return nil, fmt.Errorf("update bank matched batch error: %w", err)
	}

	if err := s.withdrawRepo.MarkReconciled(ctx, matchedIDs); err != nil {
		return nil, fmt.Errorf("mark withdraw reconciled error: %w", err)
	}

	if err := s.reconcileRepo.BatchInsertDiffs(ctx, diffs); err != nil {
		return nil, fmt.Errorf("insert diffs error: %w", err)
	}

	if err := s.reconcileRepo.UpdateLogFinished(ctx, batchNo, matchedCount, len(diffs), round2(matchedAmt)); err != nil {
		return nil, fmt.Errorf("update log finished error: %w", err)
	}

	out := make([]ReconcileDiffOut, 0, len(diffs))
	for _, d := range diffs {
		out = append(out, toReconcileDiffOut(d))
	}

	return &ReconcileResult{
		BatchNo:         batchNo,
		ReconcileDate:   date,
		TotalSystem:     len(systemWithdrawals),
		TotalBank:       len(debitStmts),
		MatchedCount:    matchedCount,
		SystemTotalAmt:  sysTotal,
		BankTotalAmt:    bankTotal,
		MatchedTotalAmt: round2(matchedAmt),
		DiffCount:       len(diffs),
		DiffList:        out,
		Status:          2,
		CreatedAt:       now.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *reconcileService) buildBankStatements(ctx context.Context, inputs []BankStatementInput, batchNo string) ([]models.BankStatement, error) {
	result := make([]models.BankStatement, 0, len(inputs))
	refSet := make(map[string]struct{}, len(inputs))
	for i := range inputs {
		in := inputs[i]
		if in.BankRefNo == "" {
			return nil, fmt.Errorf("statement index %d has empty bank_ref_no", i)
		}
		if _, dup := refSet[in.BankRefNo]; dup {
			return nil, fmt.Errorf("duplicated bank_ref_no: %s", in.BankRefNo)
		}
		refSet[in.BankRefNo] = struct{}{}
		trxTime, err := parseTrxTime(in.TrxDate, in.TrxTime)
		if err != nil {
			return nil, fmt.Errorf("parse statement %d time error: %w", i, err)
		}
		result = append(result, models.BankStatement{
			ID:          s.idGen.NextID(),
			BatchNo:     batchNo,
			BankRefNo:   in.BankRefNo,
			TrxDate:     in.TrxDate,
			TrxTime:     trxTime,
			FlowType:    in.FlowType,
			Amount:      round2(in.Amount),
			Balance:     round2(in.Balance),
			OppBankCard: in.OppBankCard,
			OppName:     in.OppName,
			Summary:     in.Summary,
		})
	}
	return result, nil
}

func parseTrxTime(date, tm string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04:05",
		"20060102150405",
	}
	combined := date + " " + tm
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, combined, time.Local); err == nil {
			return t, nil
		}
	}
	if len(tm) >= len("15:04:05") {
		return time.ParseInLocation("2006-01-02 15:04:05", combined, time.Local)
	}
	return time.ParseInLocation("2006-01-02 15:04", combined, time.Local)
}

func filterDebitOnly(list []models.BankStatement) []models.BankStatement {
	out := make([]models.BankStatement, 0, len(list))
	for i := range list {
		if list[i].FlowType == models.BankFlowTypeDebit {
			out = append(out, list[i])
		}
	}
	return out
}

func sumAmounts(ws []models.Withdrawal, bs []models.BankStatement) (sysTotal, bankTotal float64) {
	for i := range ws {
		sysTotal += ws[i].ActualAmount
	}
	for i := range bs {
		bankTotal += bs[i].Amount
	}
	return round2(sysTotal), round2(bankTotal)
}

type idx struct{}

func wl(_ []models.BankStatement) idx { return idx{} }

func indexKey(card string, amt float64) string {
	return fmt.Sprintf("%s_%.2f", card, amt)
}

func amtKey(amt float64) string {
	return fmt.Sprintf("%.2f", amt)
}

func buildCardAmtIndex(stmts []models.BankStatement, used map[int64]struct{}) map[string][]int {
	m := make(map[string][]int, len(stmts))
	for i := range stmts {
		s := stmts[i]
		if _, ok := used[s.ID]; ok {
			continue
		}
		key := indexKey(s.OppBankCard, s.Amount)
		m[key] = append(m[key], i)
	}
	return m
}

func buildCardIndex(stmts []models.BankStatement) map[string][]*models.BankStatement {
	m := make(map[string][]*models.BankStatement, len(stmts))
	for i := range stmts {
		s := &stmts[i]
		m[s.OppBankCard] = append(m[s.OppBankCard], s)
	}
	return m
}

func buildAmtIndex(stmts []models.BankStatement) map[string][]*models.BankStatement {
	m := make(map[string][]*models.BankStatement, len(stmts))
	for i := range stmts {
		s := &stmts[i]
		key := amtKey(s.Amount)
		m[key] = append(m[key], s)
	}
	return m
}

func filterUnmatched(ws []models.Withdrawal, matched map[int64]struct{}) []models.Withdrawal {
	out := make([]models.Withdrawal, 0, len(ws))
	for i := range ws {
		if _, ok := matched[ws[i].ID]; !ok {
			out = append(out, ws[i])
		}
	}
	return out
}

func filterBankUnmatched(bs []models.BankStatement, matched map[int64]struct{}) []models.BankStatement {
	out := make([]models.BankStatement, 0, len(bs))
	for i := range bs {
		if _, ok := matched[bs[i].ID]; !ok {
			out = append(out, bs[i])
		}
	}
	return out
}

func (s *reconcileService) buildDiffs(
	ws []models.Withdrawal,
	bs []models.BankStatement,
	matchedSys map[int64]struct{},
	matchedBank map[int64]struct{},
	matchedMap map[int64]int64,
	batchNo string,
	_ idx,
) []models.ReconcileDiff {
	diffs := make([]models.ReconcileDiff, 0)

	sysByID := make(map[int64]*models.Withdrawal, len(ws))
	for i := range ws {
		w := &ws[i]
		sysByID[w.ID] = w
	}
	bankByID := make(map[int64]*models.BankStatement, len(bs))
	for i := range bs {
		b := &bs[i]
		bankByID[b.ID] = b
	}

	// 检查已匹配但金额/卡号不一致的（兜底匹配产生的潜在差异）
	for bankID, sysID := range matchedMap {
		w, okW := sysByID[sysID]
		b, okB := bankByID[bankID]
		if !okW || !okB {
			continue
		}
		amtDiff := math.Abs(w.ActualAmount - b.Amount)
		cardDiff := w.BankCard != b.OppBankCard
		if amtDiff > amountEpsilon && cardDiff {
			dAmt := round2(b.Amount - w.ActualAmount)
			diffs = append(diffs, models.ReconcileDiff{
				ID:           s.idGen.NextID(),
				BatchNo:      batchNo,
				DiffType:     models.DiffTypeAmountMismatch,
				WithdrawID:   &w.ID,
				WithdrawNo:   &w.WithdrawNo,
				BankStmtID:   &b.ID,
				BankRefNo:    &b.BankRefNo,
				SystemAmount: &w.ActualAmount,
				BankAmount:   &b.Amount,
				DiffAmount:   &dAmt,
				SystemCard:   &w.BankCard,
				BankCard:     &b.OppBankCard,
				Remark:       "金额+卡号双不匹配",
			})
		} else if amtDiff > amountEpsilon {
			dAmt := round2(b.Amount - w.ActualAmount)
			diffs = append(diffs, models.ReconcileDiff{
				ID:           s.idGen.NextID(),
				BatchNo:      batchNo,
				DiffType:     models.DiffTypeAmountMismatch,
				WithdrawID:   &w.ID,
				WithdrawNo:   &w.WithdrawNo,
				BankStmtID:   &b.ID,
				BankRefNo:    &b.BankRefNo,
				SystemAmount: &w.ActualAmount,
				BankAmount:   &b.Amount,
				DiffAmount:   &dAmt,
				SystemCard:   &w.BankCard,
				BankCard:     &b.OppBankCard,
				Remark:       "金额不匹配",
			})
		} else if cardDiff {
			dAmt := 0.0
			diffs = append(diffs, models.ReconcileDiff{
				ID:           s.idGen.NextID(),
				BatchNo:      batchNo,
				DiffType:     models.DiffTypeCardMismatch,
				WithdrawID:   &w.ID,
				WithdrawNo:   &w.WithdrawNo,
				BankStmtID:   &b.ID,
				BankRefNo:    &b.BankRefNo,
				SystemAmount: &w.ActualAmount,
				BankAmount:   &b.Amount,
				DiffAmount:   &dAmt,
				SystemCard:   &w.BankCard,
				BankCard:     &b.OppBankCard,
				Remark:       "收款卡号不匹配",
			})
		}
	}

	// 系统侧未匹配
	for i := range ws {
		w := ws[i]
		if _, ok := matchedSys[w.ID]; ok {
			continue
		}
		zero := 0.0
		dAmt := w.ActualAmount
		diffs = append(diffs, models.ReconcileDiff{
			ID:           s.idGen.NextID(),
			BatchNo:      batchNo,
			DiffType:     models.DiffTypeSystemOnly,
			WithdrawID:   &w.ID,
			WithdrawNo:   &w.WithdrawNo,
			SystemAmount: &w.ActualAmount,
			BankAmount:   &zero,
			DiffAmount:   &dAmt,
			SystemCard:   &w.BankCard,
			Remark:       "系统有记录但银行流水缺失，疑似打款失败或延迟",
		})
	}

	// 银行侧未匹配
	for i := range bs {
		b := bs[i]
		if _, ok := matchedBank[b.ID]; ok {
			continue
		}
		zero := 0.0
		dAmt := -b.Amount
		diffs = append(diffs, models.ReconcileDiff{
			ID:           s.idGen.NextID(),
			BatchNo:      batchNo,
			DiffType:     models.DiffTypeBankOnly,
			BankStmtID:   &b.ID,
			BankRefNo:    &b.BankRefNo,
			SystemAmount: &zero,
			BankAmount:   &b.Amount,
			DiffAmount:   &dAmt,
			BankCard:     &b.OppBankCard,
			Remark:       "银行有流水但系统无对应提现，疑似非平台交易或订单漏登",
		})
	}

	return diffs
}

func (s *reconcileService) GetResult(ctx context.Context, batchNo string) (*ReconcileResult, error) {
	log, err := s.reconcileRepo.GetLogByBatchNo(ctx, batchNo)
	if err != nil {
		return nil, err
	}
	if log == nil {
		return nil, nil
	}
	diffList, err := s.reconcileRepo.ListDiffsByBatchNo(ctx, batchNo)
	if err != nil {
		return nil, fmt.Errorf("list diffs error: %w", err)
	}
	out := make([]ReconcileDiffOut, 0, len(diffList))
	for _, d := range diffList {
		out = append(out, toReconcileDiffOut(d))
	}
	var finishedAt string
	if log.FinishedAt != nil {
		finishedAt = log.FinishedAt.Format("2006-01-02 15:04:05")
	}
	_ = finishedAt
	return &ReconcileResult{
		BatchNo:         log.BatchNo,
		ReconcileDate:   log.ReconcileDate,
		TotalSystem:     log.TotalSystem,
		TotalBank:       log.TotalBank,
		MatchedCount:    log.MatchedCount,
		SystemTotalAmt:  log.SystemTotalAmt,
		BankTotalAmt:    log.BankTotalAmt,
		MatchedTotalAmt: log.MatchedTotalAmt,
		DiffCount:       log.DiffCount,
		DiffList:        out,
		Status:          log.Status,
		CreatedAt:       log.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}
