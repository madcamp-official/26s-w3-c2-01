package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

type PurgeResult struct {
	Transaction domain.CleanupTransaction
	DryRun      bool
	Candidates  []string
}

type PurgeService struct {
	transactions CleanupTransactionRepository
	now          func() time.Time
}

func NewPurgeService(transactions CleanupTransactionRepository) *PurgeService {
	return &PurgeService{transactions: transactions, now: time.Now}
}

func (s *PurgeService) Purge(ctx context.Context, transactionID string, quarantineDays int, execute bool) (PurgeResult, error) {
	transaction, err := s.transactions.FindByID(ctx, transactionID)
	if err != nil {
		return PurgeResult{}, err
	}
	if transaction.Status != domain.TransactionQuarantined && transaction.Status != domain.TransactionPartiallyQuarantined {
		return PurgeResult{}, fmt.Errorf("transaction %q is %s, not quarantined", transactionID, transaction.Status)
	}
	if transaction.FinishedAt == nil || transaction.FinishedAt.After(s.now().UTC().AddDate(0, 0, -quarantineDays)) {
		return PurgeResult{}, fmt.Errorf("transaction %q has not reached the %d-day quarantine retention period", transactionID, quarantineDays)
	}

	result := PurgeResult{Transaction: transaction, DryRun: !execute}
	for i := range transaction.Items {
		item := &transaction.Items[i]
		if item.Status != domain.TransactionItemMoved {
			continue
		}
		if err := validatePurgeItem(transaction.ID, *item); err != nil {
			if execute {
				item.Status = domain.TransactionItemFailed
				item.Reason = err.Error()
			}
			continue
		}
		result.Candidates = append(result.Candidates, item.QuarantinePath)
		if !execute {
			continue
		}
		if info, err := os.Lstat(item.QuarantinePath); err != nil {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		} else if scanner.IsLinkLike(info) {
			item.Status = domain.TransactionItemFailed
			item.Reason = "quarantine item became a link or reparse point"
			continue
		}
		if err := os.RemoveAll(item.QuarantinePath); err != nil {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		}
		item.Status = domain.TransactionItemPurged
		item.Reason = ""
	}
	if !execute {
		return result, nil
	}

	var purged, remaining int
	manifestPaths := map[string]bool{}
	for _, item := range transaction.Items {
		if item.Status == domain.TransactionItemPurged {
			purged++
		} else if item.Status == domain.TransactionItemMoved || item.Status == domain.TransactionItemFailed {
			remaining++
		}
		if item.ManifestPath != "" {
			manifestPaths[item.ManifestPath] = true
		}
	}
	if purged == 0 {
		return PurgeResult{}, errors.New("no quarantine items passed purge validation")
	}
	if remaining == 0 {
		transaction.Status = domain.TransactionPurged
	} else {
		transaction.Status = domain.TransactionPartiallyPurged
	}
	finished := s.now().UTC()
	transaction.FinishedAt = &finished
	if err := s.transactions.Update(ctx, transaction); err != nil {
		return PurgeResult{}, fmt.Errorf("record purge result: %w", err)
	}
	if transaction.Status == domain.TransactionPurged {
		for manifestPath := range manifestPaths {
			_ = os.Remove(manifestPath)
			_ = os.Remove(filepath.Dir(manifestPath))
		}
	}
	result.Transaction = transaction
	return result, nil
}

func validatePurgeItem(transactionID string, item domain.CleanupTransactionItem) error {
	if item.QuarantinePath == "" || item.ManifestPath == "" {
		return errors.New("quarantine path and manifest are required")
	}
	inside, err := pathutil.IsSameOrChild(item.QuarantinePath, filepath.Join(filepath.Dir(item.ManifestPath), "items"))
	if err != nil || !inside {
		return errors.New("quarantine item is outside its manifest items directory")
	}
	equalRoot, err := pathutil.Equal(item.QuarantinePath, filepath.Join(filepath.Dir(item.ManifestPath), "items"))
	if err != nil || equalRoot {
		return errors.New("quarantine item must be below the manifest items directory")
	}
	data, err := os.ReadFile(item.ManifestPath)
	if err != nil {
		return fmt.Errorf("read quarantine manifest: %w", err)
	}
	var manifest safety.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode quarantine manifest: %w", err)
	}
	if manifest.SchemaVersion != safety.ManifestVersion || manifest.TransactionID != transactionID {
		return errors.New("quarantine manifest identity does not match transaction")
	}
	for _, recorded := range manifest.Items {
		if recorded.ID == item.ID && recorded.QuarantinePath == item.QuarantinePath {
			return nil
		}
	}
	return errors.New("quarantine item is not present in manifest")
}
