package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// GetExtension retrieves a single extension by extension number and domain
func (db *DB) GetExtension(ctx context.Context, extension, domain string) (*models.Extension, error) {
	query := `
		SELECT
			e.id, e.domain_id, e.extension, e.type, e.display_name,
			e.email, e.sip_password, e.sip_ha1, e.sip_ha1b,
			e.vm_password, e.vm_email, e.active, e.max_concurrent,
			e.call_timeout, e.created_at, e.updated_at,
			d.domain
		FROM voip.extensions e
		INNER JOIN voip.domains d ON e.domain_id = d.id
		WHERE e.extension = $1 AND d.domain = $2
	`

	var ext models.Extension
	err := db.QueryRowContext(ctx, query, extension, domain).Scan(
		&ext.ID, &ext.DomainID, &ext.Extension, &ext.Type, &ext.DisplayName,
		&ext.Email, &ext.SIPPassword, &ext.SIPHA1, &ext.SIPHA1B,
		&ext.VMPassword, &ext.VMEmail, &ext.Active, &ext.MaxConcurrent,
		&ext.CallTimeout, &ext.CreatedAt, &ext.UpdatedAt,
		&ext.Domain,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("extension not found: %s@%s", extension, domain)
	}
	if err != nil {
		return nil, fmt.Errorf("query extension: %w", err)
	}

	return &ext, nil
}

// GetExtensionByID retrieves a single extension by ID
func (db *DB) GetExtensionByID(ctx context.Context, id int64) (*models.Extension, error) {
	query := `
		SELECT
			e.id, e.domain_id, e.extension, e.type, e.display_name,
			e.email, e.sip_password, e.sip_ha1, e.sip_ha1b,
			e.vm_password, e.vm_email, e.active, e.max_concurrent,
			e.call_timeout, e.created_at, e.updated_at,
			d.domain
		FROM voip.extensions e
		INNER JOIN voip.domains d ON e.domain_id = d.id
		WHERE e.id = $1
	`

	var ext models.Extension
	err := db.QueryRowContext(ctx, query, id).Scan(
		&ext.ID, &ext.DomainID, &ext.Extension, &ext.Type, &ext.DisplayName,
		&ext.Email, &ext.SIPPassword, &ext.SIPHA1, &ext.SIPHA1B,
		&ext.VMPassword, &ext.VMEmail, &ext.Active, &ext.MaxConcurrent,
		&ext.CallTimeout, &ext.CreatedAt, &ext.UpdatedAt,
		&ext.Domain,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("extension not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query extension: %w", err)
	}

	return &ext, nil
}

// ListExtensions retrieves extensions with pagination and filtering
func (db *DB) ListExtensions(ctx context.Context, domainID *int64, extType *string, active *bool, page, perPage int) (*models.ExtensionListResponse, error) {
	// Build WHERE clause
	var conditions []string
	var args []interface{}
	argPos := 1

	if domainID != nil {
		conditions = append(conditions, fmt.Sprintf("e.domain_id = $%d", argPos))
		args = append(args, *domainID)
		argPos++
	}

	if extType != nil {
		conditions = append(conditions, fmt.Sprintf("e.type = $%d", argPos))
		args = append(args, *extType)
		argPos++
	}

	if active != nil {
		conditions = append(conditions, fmt.Sprintf("e.active = $%d", argPos))
		args = append(args, *active)
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM voip.extensions e
		%s
	`, whereClause)

	var total int64
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count extensions: %w", err)
	}

	// Fetch extensions
	offset := (page - 1) * perPage
	args = append(args, perPage, offset)

	query := fmt.Sprintf(`
		SELECT
			e.id, e.domain_id, e.extension, e.type, e.display_name,
			e.email, e.sip_password, e.sip_ha1, e.sip_ha1b,
			e.vm_password, e.vm_email, e.active, e.max_concurrent,
			e.call_timeout, e.created_at, e.updated_at,
			d.domain
		FROM voip.extensions e
		INNER JOIN voip.domains d ON e.domain_id = d.id
		%s
		ORDER BY e.extension
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query extensions: %w", err)
	}
	defer rows.Close()

	var extensions []*models.Extension
	for rows.Next() {
		var ext models.Extension
		if err := rows.Scan(
			&ext.ID, &ext.DomainID, &ext.Extension, &ext.Type, &ext.DisplayName,
			&ext.Email, &ext.SIPPassword, &ext.SIPHA1, &ext.SIPHA1B,
			&ext.VMPassword, &ext.VMEmail, &ext.Active, &ext.MaxConcurrent,
			&ext.CallTimeout, &ext.CreatedAt, &ext.UpdatedAt,
			&ext.Domain,
		); err != nil {
			return nil, fmt.Errorf("scan extension: %w", err)
		}
		extensions = append(extensions, &ext)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return &models.ExtensionListResponse{
		Extensions: extensions,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
	}, nil
}

// CreateExtension creates a new extension
func (db *DB) CreateExtension(ctx context.Context, req *models.ExtensionCreateRequest) (*models.Extension, error) {
	// Note: HA1 hashes will be calculated by database trigger
	query := `
		INSERT INTO voip.extensions (
			domain_id, extension, type, display_name, email,
			sip_password, vm_password, vm_email, active,
			max_concurrent, call_timeout
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	var ext models.Extension
	ext.DomainID = req.DomainID
	ext.Extension = req.Extension
	ext.Type = req.Type
	ext.DisplayName = req.DisplayName
	ext.Email = req.Email
	ext.SIPPassword = req.SIPPassword
	ext.VMPassword = req.VMPassword
	ext.VMEmail = req.VMEmail
	ext.Active = req.Active
	ext.MaxConcurrent = req.MaxConcurrent
	ext.CallTimeout = req.CallTimeout

	err := db.QueryRowContext(ctx, query,
		req.DomainID, req.Extension, req.Type, req.DisplayName, req.Email,
		req.SIPPassword, req.VMPassword, req.VMEmail, req.Active,
		req.MaxConcurrent, req.CallTimeout,
	).Scan(&ext.ID, &ext.CreatedAt, &ext.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert extension: %w", err)
	}

	// Fetch full extension with domain info
	return db.GetExtensionByID(ctx, ext.ID)
}

// UpdateExtension updates an existing extension
func (db *DB) UpdateExtension(ctx context.Context, id int64, req *models.ExtensionUpdateRequest) (*models.Extension, error) {
	var setClauses []string
	var args []interface{}
	argPos := 1

	if req.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argPos))
		args = append(args, *req.DisplayName)
		argPos++
	}

	if req.Email != nil {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argPos))
		args = append(args, *req.Email)
		argPos++
	}

	if req.VMPassword != nil {
		setClauses = append(setClauses, fmt.Sprintf("vm_password = $%d", argPos))
		args = append(args, *req.VMPassword)
		argPos++
	}

	if req.VMEmail != nil {
		setClauses = append(setClauses, fmt.Sprintf("vm_email = $%d", argPos))
		args = append(args, *req.VMEmail)
		argPos++
	}

	if req.Active != nil {
		setClauses = append(setClauses, fmt.Sprintf("active = $%d", argPos))
		args = append(args, *req.Active)
		argPos++
	}

	if req.MaxConcurrent != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_concurrent = $%d", argPos))
		args = append(args, *req.MaxConcurrent)
		argPos++
	}

	if req.CallTimeout != nil {
		setClauses = append(setClauses, fmt.Sprintf("call_timeout = $%d", argPos))
		args = append(args, *req.CallTimeout)
		argPos++
	}

	if len(setClauses) == 0 {
		return db.GetExtensionByID(ctx, id)
	}

	// Add updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	// Add ID for WHERE clause
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE voip.extensions
		SET %s
		WHERE id = $%d
	`, strings.Join(setClauses, ", "), argPos)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update extension: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("extension not found: %d", id)
	}

	return db.GetExtensionByID(ctx, id)
}

// UpdateExtensionPassword updates extension's SIP password
func (db *DB) UpdateExtensionPassword(ctx context.Context, id int64, newPassword string) error {
	// HA1 hashes will be recalculated by database trigger
	query := `
		UPDATE voip.extensions
		SET sip_password = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := db.ExecContext(ctx, query, newPassword, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("extension not found: %d", id)
	}

	return nil
}

// DeleteExtension soft-deletes an extension by marking it as inactive
func (db *DB) DeleteExtension(ctx context.Context, id int64) error {
	query := `
		UPDATE voip.extensions
		SET active = false, updated_at = $1
		WHERE id = $2
	`

	result, err := db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("delete extension: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("extension not found: %d", id)
	}

	return nil
}

// GetDomain retrieves a domain by ID
func (db *DB) GetDomain(ctx context.Context, id int64) (*models.Domain, error) {
	query := `
		SELECT id, domain, active, created_at, updated_at
		FROM voip.domains
		WHERE id = $1
	`

	var domain models.Domain
	err := db.QueryRowContext(ctx, query, id).Scan(
		&domain.ID, &domain.Domain, &domain.Active,
		&domain.CreatedAt, &domain.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("domain not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query domain: %w", err)
	}

	return &domain, nil
}

// GetDomainByName retrieves a domain by name
func (db *DB) GetDomainByName(ctx context.Context, name string) (*models.Domain, error) {
	query := `
		SELECT id, domain, active, created_at, updated_at
		FROM voip.domains
		WHERE domain = $1
	`

	var domain models.Domain
	err := db.QueryRowContext(ctx, query, name).Scan(
		&domain.ID, &domain.Domain, &domain.Active,
		&domain.CreatedAt, &domain.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("domain not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("query domain: %w", err)
	}

	return &domain, nil
}
