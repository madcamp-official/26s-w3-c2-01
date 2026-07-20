package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
)

type CleanupService struct {
	plans        CleanupPlanRepository
	resources    ResourceRepository
	projects     ProjectRepository
	transactions CleanupTransactionRepository
	validator    safety.CleanupValidator
	quarantine   safety.QuarantineEngine
	now          func() time.Time
}

func NewCleanupService(plans CleanupPlanRepository, resources ResourceRepository, projects ProjectRepository, transactions CleanupTransactionRepository, validator safety.CleanupValidator, quarantine safety.QuarantineEngine) *CleanupService {
	return &CleanupService{plans: plans, resources: resources, projects: projects, transactions: transactions, validator: validator, quarantine: quarantine, now: time.Now}
}

func (s *CleanupService) Execute(ctx context.Context, planID string) (domain.CleanupTransaction, error) {
	plan, err := s.plans.FindByID(ctx, planID)
	if err != nil {
		return domain.CleanupTransaction{}, err
	}
	id, err := newTransactionID(s.now)
	if err != nil {
		return domain.CleanupTransaction{}, err
	}
	transaction := domain.CleanupTransaction{ID: id, PlanID: plan.ID, StartedAt: s.now().UTC(), Status: domain.TransactionRunning}
	for _, planItem := range plan.Items {
		item := domain.CleanupTransactionItem{ID: id + ":" + planItem.ID, PlanItemID: planItem.ID, ResourceID: planItem.ResourceID, OriginalPath: planItem.NormalizedPath, ExpectedSize: planItem.ExpectedSize, Status: domain.TransactionItemPending}
		resource, findErr := s.resources.FindByID(ctx, planItem.ResourceID)
		if findErr != nil {
			item.Status = domain.TransactionItemSkipped
			item.Reason = "resource no longer exists: " + findErr.Error()
			transaction.Items = append(transaction.Items, item)
			continue
		}
		var ownerRoot string
		if planItem.OwnerProjectID != "" {
			project, projectErr := s.projects.FindByID(ctx, planItem.OwnerProjectID)
			if projectErr == nil {
				ownerRoot = project.NormalizedRootPath
			}
		}
		if _, validationErr := s.validator.Validate(ctx, planItem, resource, ownerRoot); validationErr != nil {
			item.Status = domain.TransactionItemSkipped
			item.Reason = validationErr.Error()
		}
		transaction.Items = append(transaction.Items, item)
	}
	if err := s.quarantine.Prepare(&transaction); err != nil {
		return domain.CleanupTransaction{}, fmt.Errorf("prepare quarantine manifests: %w", err)
	}
	s.quarantine.Move(ctx, &transaction)
	finishTransaction(&transaction, s.now().UTC(), false)
	if err := s.transactions.Create(ctx, transaction); err != nil {
		return transaction, fmt.Errorf("record cleanup transaction (recover from disk manifest): %w", err)
	}
	return transaction, nil
}

func (s *CleanupService) Restore(ctx context.Context, transactionID string) (domain.CleanupTransaction, error) {
	transaction, err := s.transactions.FindByID(ctx, transactionID)
	if err != nil {
		return domain.CleanupTransaction{}, err
	}
	s.quarantine.Restore(ctx, &transaction)
	finishTransaction(&transaction, s.now().UTC(), true)
	if err := s.transactions.Update(ctx, transaction); err != nil {
		return transaction, fmt.Errorf("record restore result: %w", err)
	}
	return transaction, nil
}

func finishTransaction(transaction *domain.CleanupTransaction, finished time.Time, restoring bool) {
	var success, other int
	for _, item := range transaction.Items {
		if (!restoring && item.Status == domain.TransactionItemMoved) || (restoring && item.Status == domain.TransactionItemRestored) {
			success++
		} else {
			other++
		}
	}
	if restoring {
		switch {
		case success > 0 && other == 0:
			transaction.Status = domain.TransactionRestored
		case success > 0:
			transaction.Status = domain.TransactionPartiallyRestored
		default:
			transaction.Status = domain.TransactionFailed
		}
	} else {
		switch {
		case success > 0 && other == 0:
			transaction.Status = domain.TransactionQuarantined
		case success > 0:
			transaction.Status = domain.TransactionPartiallyQuarantined
		default:
			transaction.Status = domain.TransactionFailed
		}
	}
	transaction.FinishedAt = &finished
}

func newTransactionID(now func() time.Time) (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return fmt.Sprintf("tx-%s-%s", now().UTC().Format("20060102-150405"), hex.EncodeToString(suffix)), nil
}
