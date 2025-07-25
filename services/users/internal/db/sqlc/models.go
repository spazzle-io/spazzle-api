// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0

package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type User struct {
	ID                uuid.UUID          `json:"id"`
	WalletAddress     string             `json:"wallet_address"`
	GamerTag          pgtype.Text        `json:"gamer_tag"`
	EnsName           pgtype.Text        `json:"ens_name"`
	EnsAvatarUri      pgtype.Text        `json:"ens_avatar_uri"`
	EnsImageUrl       pgtype.Text        `json:"ens_image_url"`
	EnsLastResolvedAt pgtype.Timestamptz `json:"ens_last_resolved_at"`
	CreatedAt         time.Time          `json:"created_at"`
}
