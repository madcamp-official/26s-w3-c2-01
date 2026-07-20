package safety

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

const ManifestVersion = 1

type Manifest struct {
	SchemaVersion int                             `json:"schema_version"`
	TransactionID string                          `json:"transaction_id"`
	PlanID        string                          `json:"plan_id"`
	UpdatedAt     time.Time                       `json:"updated_at"`
	Items         []domain.CleanupTransactionItem `json:"items"`
}

type QuarantineEngine struct {
	RootForPath func(originalPath, transactionID string) string
	Now         func() time.Time
}

func (e QuarantineEngine) Prepare(transaction *domain.CleanupTransaction) error {
	for i := range transaction.Items {
		root := e.root(transaction.Items[i].OriginalPath, transaction.ID)
		digest := sha256.Sum256([]byte(transaction.Items[i].ID))
		name := hex.EncodeToString(digest[:6]) + "-" + filepath.Base(transaction.Items[i].OriginalPath)
		transaction.Items[i].QuarantinePath = filepath.Join(root, "items", name)
		transaction.Items[i].ManifestPath = filepath.Join(root, "manifest.json")
	}
	return e.writeManifests(*transaction)
}

func (e QuarantineEngine) Move(_ context.Context, transaction *domain.CleanupTransaction) {
	for i := range transaction.Items {
		item := &transaction.Items[i]
		if item.Status != domain.TransactionItemPending {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(item.QuarantinePath), 0o700); err != nil {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		}
		if err := os.Rename(item.OriginalPath, item.QuarantinePath); err != nil {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		}
		item.Status = domain.TransactionItemMoved
		if err := e.writeManifests(*transaction); err != nil {
			item.Reason = "moved; manifest update failed: " + err.Error()
		}
	}
}

func (e QuarantineEngine) Restore(_ context.Context, transaction *domain.CleanupTransaction) {
	for i := range transaction.Items {
		item := &transaction.Items[i]
		if item.Status != domain.TransactionItemMoved && item.Status != domain.TransactionItemSkipped {
			continue
		}
		if _, err := os.Lstat(item.QuarantinePath); err != nil {
			if item.Status == domain.TransactionItemSkipped { continue }
			item.Status = domain.TransactionItemFailed; item.Reason = "quarantine item missing: " + err.Error(); continue
		}
		if _, err := os.Lstat(item.OriginalPath); err == nil {
			item.Status = domain.TransactionItemSkipped
			item.Reason = "original path already exists"
			continue
		} else if !os.IsNotExist(err) {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		}
		if err := os.MkdirAll(filepath.Dir(item.OriginalPath), 0o755); err != nil {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		}
		if err := os.Rename(item.QuarantinePath, item.OriginalPath); err != nil {
			item.Status = domain.TransactionItemFailed
			item.Reason = err.Error()
			continue
		}
		item.Status = domain.TransactionItemRestored
		if err := e.writeManifests(*transaction); err != nil {
			item.Reason = "restored; manifest update failed: " + err.Error()
		}
	}
}

func (e QuarantineEngine) root(path, transactionID string) string {
	if e.RootForPath != nil {
		return e.RootForPath(path, transactionID)
	}
	volume := filepath.VolumeName(path)
	if volume != "" {
		return filepath.Join(volume+string(filepath.Separator), ".libra-quarantine", transactionID)
	}
	return filepath.Join(filepath.Dir(path), ".libra-quarantine", transactionID)
}

func (e QuarantineEngine) writeManifests(transaction domain.CleanupTransaction) error {
	byManifest := make(map[string][]domain.CleanupTransactionItem)
	for _, item := range transaction.Items {
		byManifest[item.ManifestPath] = append(byManifest[item.ManifestPath], item)
	}
	paths := make([]string, 0, len(byManifest))
	for path := range byManifest {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	now := time.Now
	if e.Now != nil {
		now = e.Now
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		manifest := Manifest{SchemaVersion: ManifestVersion, TransactionID: transaction.ID, PlanID: transaction.PlanID, UpdatedAt: now().UTC(), Items: byManifest[path]}
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		temporary := path + ".tmp"
		if err := os.WriteFile(temporary, data, 0o600); err != nil {
			return err
		}
		if err := os.Rename(temporary, path); err != nil {
			return fmt.Errorf("commit manifest %q: %w", path, err)
		}
	}
	return nil
}
