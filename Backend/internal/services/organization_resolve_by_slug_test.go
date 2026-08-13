package services_test

import (
	"errors"
	"testing"

	"Backend/internal/services/organization"

	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestOrganizationService_ResolveBySlug(t *testing.T) {
	cols := []string{"id", "name", "slug", "status"}

	tests := []struct {
		name    string
		slug    string
		rows    *sqlmock.Rows
		wantID  uint
		wantErr error
	}{
		{
			name:   "active organization resolves",
			slug:   "oo-univ",
			rows:   sqlmock.NewRows(cols).AddRow(5, "OO大学", "oo-univ", "active"),
			wantID: 5,
		},
		{
			name:    "disabled organization is rejected",
			slug:    "closed-univ",
			rows:    sqlmock.NewRows(cols).AddRow(6, "旧学園", "closed-univ", "disabled"),
			wantErr: services.ErrOrganizationDisabled,
		},
		{
			name:    "unknown slug is rejected",
			slug:    "no-such-univ",
			rows:    sqlmock.NewRows(cols),
			wantErr: organization.ErrOrganizationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newOrgTestDB(t)
			repo := repositories.NewOrganizationRepository(db)
			svc := organization.NewOrganizationService(repo)

			mock.ExpectQuery("SELECT \\* FROM `organizations` WHERE slug = \\? ORDER BY `organizations`.`id` LIMIT \\?").
				WithArgs(tt.slug, 1).
				WillReturnRows(tt.rows)

			org, err := svc.ResolveBySlug(tt.slug)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v want %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if org.ID != tt.wantID {
					t.Fatalf("org.ID = %d want %d", org.ID, tt.wantID)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("sql: %v", err)
			}
		})
	}
}
