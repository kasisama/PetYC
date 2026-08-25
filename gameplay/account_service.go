package gameplay

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"qq-pet-saas/core"
	"qq-pet-saas/models"
)

var errIdentityClaimed = errors.New("identity was claimed concurrently")

type AccountService struct {
	DB *gorm.DB
}

func NewAccountService(db *gorm.DB) *AccountService {
	return &AccountService{DB: db}
}

// IdentityScope freezes the v2 identity contract: OneBot identities follow a
// player across groups, while official QQ identities remain scoped to the
// official scene supplied by the gateway.
func IdentityScope(event core.InboundEvent) string {
	if event.Platform == core.PlatformOneBot {
		return "*"
	}
	return event.SpaceID
}

func (service *AccountService) Resolve(ctx context.Context, event core.InboundEvent) (*models.PlayerAccount, error) {
	if service == nil || service.DB == nil {
		return nil, ErrDatabaseUnavailable
	}
	if event.Platform == "" || event.AppID == "" || event.SceneType == "" || event.ActorID == "" {
		return nil, ErrIdentityRequired
	}
	if account, err := service.find(ctx, event); err == nil {
		return account, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var account models.PlayerAccount
	err := WithTransactionRetry(ctx, service.DB, func(tx *gorm.DB) error {
		if existing, lookupErr := findAccount(tx, event); lookupErr == nil {
			account = *existing
			return nil
		} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}

		account = models.PlayerAccount{ID: uuid.NewString()}
		if err := tx.Create(&account).Error; err != nil {
			return err
		}
		identity := identityFor(event, account.ID)
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&identity)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errIdentityClaimed
		}
		return nil
	})
	if errors.Is(err, errIdentityClaimed) {
		return service.find(ctx, event)
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (service *AccountService) find(ctx context.Context, event core.InboundEvent) (*models.PlayerAccount, error) {
	return findAccount(service.DB.WithContext(ctx), event)
}

func findAccount(db *gorm.DB, event core.InboundEvent) (*models.PlayerAccount, error) {
	var identity models.PlayerIdentity
	key := identityFor(event, "")
	err := db.Where("platform = ? AND app_id = ? AND scene_type = ? AND scope_id = ? AND subject_id = ?",
		key.Platform, key.AppID, key.SceneType, key.ScopeID, key.SubjectID).First(&identity).Error
	if err != nil {
		return nil, err
	}
	var account models.PlayerAccount
	if err = db.First(&account, "id = ?", identity.AccountID).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func identityFor(event core.InboundEvent, accountID string) models.PlayerIdentity {
	return models.PlayerIdentity{
		AccountID: accountID,
		Platform:  string(event.Platform),
		AppID:     event.AppID,
		SceneType: string(event.SceneType),
		ScopeID:   IdentityScope(event),
		SubjectID: event.ActorID,
	}
}
