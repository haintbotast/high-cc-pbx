package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/yourusername/high-cc-pbx/voip-admin/internal/models"
)

// GetQueue retrieves a queue by ID
func (db *DB) GetQueue(ctx context.Context, id int64) (*models.Queue, error) {
	query := `
		SELECT
			q.id, q.name, q.extension, q.domain_id, q.strategy, q.moh,
			q.record_template, q.time_base_score, q.max_wait_time,
			q.max_wait_time_no_agent, q.tier_rules_apply, q.tier_rule_wait_second,
			q.discard_abandoned_after, q.abandoned_resume_allowed, q.active,
			q.created_at, q.updated_at, d.domain
		FROM voip.queues q
		INNER JOIN voip.domains d ON q.domain_id = d.id
		WHERE q.id = $1
	`

	var queue models.Queue
	err := db.QueryRowContext(ctx, query, id).Scan(
		&queue.ID, &queue.Name, &queue.Extension, &queue.DomainID, &queue.Strategy, &queue.Moh,
		&queue.RecordTemplate, &queue.TimeBaseScore, &queue.MaxWaitTime,
		&queue.MaxWaitTimeNoAgent, &queue.TierRulesApply, &queue.TierRuleWaitSecond,
		&queue.DiscardAbandonedAfter, &queue.AbandonedResumeAllowed, &queue.Active,
		&queue.CreatedAt, &queue.UpdatedAt, &queue.Domain,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query queue: %w", err)
	}

	return &queue, nil
}

// GetQueueByExtension retrieves a queue by extension and domain
func (db *DB) GetQueueByExtension(ctx context.Context, extension, domain string) (*models.Queue, error) {
	query := `
		SELECT
			q.id, q.name, q.extension, q.domain_id, q.strategy, q.moh,
			q.record_template, q.time_base_score, q.max_wait_time,
			q.max_wait_time_no_agent, q.tier_rules_apply, q.tier_rule_wait_second,
			q.discard_abandoned_after, q.abandoned_resume_allowed, q.active,
			q.created_at, q.updated_at, d.domain
		FROM voip.queues q
		INNER JOIN voip.domains d ON q.domain_id = d.id
		WHERE q.extension = $1 AND d.domain = $2
	`

	var queue models.Queue
	err := db.QueryRowContext(ctx, query, extension, domain).Scan(
		&queue.ID, &queue.Name, &queue.Extension, &queue.DomainID, &queue.Strategy, &queue.Moh,
		&queue.RecordTemplate, &queue.TimeBaseScore, &queue.MaxWaitTime,
		&queue.MaxWaitTimeNoAgent, &queue.TierRulesApply, &queue.TierRuleWaitSecond,
		&queue.DiscardAbandonedAfter, &queue.AbandonedResumeAllowed, &queue.Active,
		&queue.CreatedAt, &queue.UpdatedAt, &queue.Domain,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue not found: %s@%s", extension, domain)
	}
	if err != nil {
		return nil, fmt.Errorf("query queue: %w", err)
	}

	return &queue, nil
}

// ListQueues retrieves queues with filtering
func (db *DB) ListQueues(ctx context.Context, domainID *int64, active *bool) ([]*models.Queue, error) {
	var conditions []string
	var args []interface{}
	argPos := 1

	if domainID != nil {
		conditions = append(conditions, fmt.Sprintf("q.domain_id = $%d", argPos))
		args = append(args, *domainID)
		argPos++
	}

	if active != nil {
		conditions = append(conditions, fmt.Sprintf("q.active = $%d", argPos))
		args = append(args, *active)
		argPos++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			q.id, q.name, q.extension, q.domain_id, q.strategy, q.moh,
			q.record_template, q.time_base_score, q.max_wait_time,
			q.max_wait_time_no_agent, q.tier_rules_apply, q.tier_rule_wait_second,
			q.discard_abandoned_after, q.abandoned_resume_allowed, q.active,
			q.created_at, q.updated_at, d.domain
		FROM voip.queues q
		INNER JOIN voip.domains d ON q.domain_id = d.id
		%s
		ORDER BY q.name
	`, whereClause)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query queues: %w", err)
	}
	defer rows.Close()

	var queues []*models.Queue
	for rows.Next() {
		var queue models.Queue
		if err := rows.Scan(
			&queue.ID, &queue.Name, &queue.Extension, &queue.DomainID, &queue.Strategy, &queue.Moh,
			&queue.RecordTemplate, &queue.TimeBaseScore, &queue.MaxWaitTime,
			&queue.MaxWaitTimeNoAgent, &queue.TierRulesApply, &queue.TierRuleWaitSecond,
			&queue.DiscardAbandonedAfter, &queue.AbandonedResumeAllowed, &queue.Active,
			&queue.CreatedAt, &queue.UpdatedAt, &queue.Domain,
		); err != nil {
			return nil, fmt.Errorf("scan queue: %w", err)
		}
		queues = append(queues, &queue)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return queues, nil
}

// CreateQueue creates a new queue
func (db *DB) CreateQueue(ctx context.Context, req *models.QueueCreateRequest) (*models.Queue, error) {
	query := `
		INSERT INTO voip.queues (
			name, extension, domain_id, strategy, moh, record_template,
			time_base_score, max_wait_time, max_wait_time_no_agent,
			tier_rules_apply, tier_rule_wait_second, discard_abandoned_after,
			abandoned_resume_allowed, active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at
	`

	var queue models.Queue
	queue.Name = req.Name
	queue.Extension = req.Extension
	queue.DomainID = req.DomainID
	queue.Strategy = req.Strategy
	queue.Moh = req.Moh
	queue.RecordTemplate = req.RecordTemplate
	queue.TimeBaseScore = req.TimeBaseScore
	queue.MaxWaitTime = req.MaxWaitTime
	queue.MaxWaitTimeNoAgent = req.MaxWaitTimeNoAgent
	queue.TierRulesApply = req.TierRulesApply
	queue.TierRuleWaitSecond = req.TierRuleWaitSecond
	queue.DiscardAbandonedAfter = req.DiscardAbandonedAfter
	queue.AbandonedResumeAllowed = req.AbandonedResumeAllowed
	queue.Active = req.Active

	err := db.QueryRowContext(ctx, query,
		req.Name, req.Extension, req.DomainID, req.Strategy, req.Moh, req.RecordTemplate,
		req.TimeBaseScore, req.MaxWaitTime, req.MaxWaitTimeNoAgent,
		req.TierRulesApply, req.TierRuleWaitSecond, req.DiscardAbandonedAfter,
		req.AbandonedResumeAllowed, req.Active,
	).Scan(&queue.ID, &queue.CreatedAt, &queue.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert queue: %w", err)
	}

	return db.GetQueue(ctx, queue.ID)
}

// UpdateQueue updates an existing queue
func (db *DB) UpdateQueue(ctx context.Context, id int64, req *models.QueueUpdateRequest) (*models.Queue, error) {
	var setClauses []string
	var args []interface{}
	argPos := 1

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *req.Name)
		argPos++
	}

	if req.Strategy != nil {
		setClauses = append(setClauses, fmt.Sprintf("strategy = $%d", argPos))
		args = append(args, *req.Strategy)
		argPos++
	}

	if req.Moh != nil {
		setClauses = append(setClauses, fmt.Sprintf("moh = $%d", argPos))
		args = append(args, *req.Moh)
		argPos++
	}

	if req.RecordTemplate != nil {
		setClauses = append(setClauses, fmt.Sprintf("record_template = $%d", argPos))
		args = append(args, *req.RecordTemplate)
		argPos++
	}

	if req.TimeBaseScore != nil {
		setClauses = append(setClauses, fmt.Sprintf("time_base_score = $%d", argPos))
		args = append(args, *req.TimeBaseScore)
		argPos++
	}

	if req.MaxWaitTime != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_wait_time = $%d", argPos))
		args = append(args, *req.MaxWaitTime)
		argPos++
	}

	if req.MaxWaitTimeNoAgent != nil {
		setClauses = append(setClauses, fmt.Sprintf("max_wait_time_no_agent = $%d", argPos))
		args = append(args, *req.MaxWaitTimeNoAgent)
		argPos++
	}

	if req.TierRulesApply != nil {
		setClauses = append(setClauses, fmt.Sprintf("tier_rules_apply = $%d", argPos))
		args = append(args, *req.TierRulesApply)
		argPos++
	}

	if req.TierRuleWaitSecond != nil {
		setClauses = append(setClauses, fmt.Sprintf("tier_rule_wait_second = $%d", argPos))
		args = append(args, *req.TierRuleWaitSecond)
		argPos++
	}

	if req.DiscardAbandonedAfter != nil {
		setClauses = append(setClauses, fmt.Sprintf("discard_abandoned_after = $%d", argPos))
		args = append(args, *req.DiscardAbandonedAfter)
		argPos++
	}

	if req.AbandonedResumeAllowed != nil {
		setClauses = append(setClauses, fmt.Sprintf("abandoned_resume_allowed = $%d", argPos))
		args = append(args, *req.AbandonedResumeAllowed)
		argPos++
	}

	if req.Active != nil {
		setClauses = append(setClauses, fmt.Sprintf("active = $%d", argPos))
		args = append(args, *req.Active)
		argPos++
	}

	if len(setClauses) == 0 {
		return db.GetQueue(ctx, id)
	}

	// Add updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	// Add ID for WHERE clause
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE voip.queues
		SET %s
		WHERE id = $%d
	`, strings.Join(setClauses, ", "), argPos)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update queue: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("queue not found: %d", id)
	}

	return db.GetQueue(ctx, id)
}

// DeleteQueue soft-deletes a queue by marking it as inactive
func (db *DB) DeleteQueue(ctx context.Context, id int64) error {
	query := `
		UPDATE voip.queues
		SET active = false, updated_at = $1
		WHERE id = $2
	`

	result, err := db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("queue not found: %d", id)
	}

	return nil
}

// GetQueueAgent retrieves a queue agent by ID
func (db *DB) GetQueueAgent(ctx context.Context, id int64) (*models.QueueAgent, error) {
	query := `
		SELECT
			qa.id, qa.queue_id, qa.extension_id, qa.state, qa.status,
			qa.tier, qa.position, qa.active, qa.created_at, qa.updated_at,
			e.extension, e.display_name
		FROM voip.queue_agents qa
		INNER JOIN voip.extensions e ON qa.extension_id = e.id
		WHERE qa.id = $1
	`

	var agent models.QueueAgent
	err := db.QueryRowContext(ctx, query, id).Scan(
		&agent.ID, &agent.QueueID, &agent.ExtensionID, &agent.State, &agent.Status,
		&agent.Tier, &agent.Position, &agent.Active, &agent.CreatedAt, &agent.UpdatedAt,
		&agent.Extension, &agent.DisplayName,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("queue agent not found: %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query queue agent: %w", err)
	}

	return &agent, nil
}

// ListQueueAgents retrieves agents for a specific queue
func (db *DB) ListQueueAgents(ctx context.Context, queueID int64, active *bool) ([]*models.QueueAgent, error) {
	conditions := []string{"qa.queue_id = $1"}
	args := []interface{}{queueID}
	argPos := 2

	if active != nil {
		conditions = append(conditions, fmt.Sprintf("qa.active = $%d", argPos))
		args = append(args, *active)
		argPos++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			qa.id, qa.queue_id, qa.extension_id, qa.state, qa.status,
			qa.tier, qa.position, qa.active, qa.created_at, qa.updated_at,
			e.extension, e.display_name
		FROM voip.queue_agents qa
		INNER JOIN voip.extensions e ON qa.extension_id = e.id
		%s
		ORDER BY qa.tier, qa.position
	`, whereClause)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query queue agents: %w", err)
	}
	defer rows.Close()

	var agents []*models.QueueAgent
	for rows.Next() {
		var agent models.QueueAgent
		if err := rows.Scan(
			&agent.ID, &agent.QueueID, &agent.ExtensionID, &agent.State, &agent.Status,
			&agent.Tier, &agent.Position, &agent.Active, &agent.CreatedAt, &agent.UpdatedAt,
			&agent.Extension, &agent.DisplayName,
		); err != nil {
			return nil, fmt.Errorf("scan queue agent: %w", err)
		}
		agents = append(agents, &agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return agents, nil
}

// CreateQueueAgent adds an agent to a queue
func (db *DB) CreateQueueAgent(ctx context.Context, req *models.QueueAgentCreateRequest) (*models.QueueAgent, error) {
	query := `
		INSERT INTO voip.queue_agents (
			queue_id, extension_id, tier, position, state, status, active
		) VALUES ($1, $2, $3, $4, $5, 'Waiting', $6)
		RETURNING id, created_at, updated_at
	`

	var agent models.QueueAgent
	agent.QueueID = req.QueueID
	agent.ExtensionID = req.ExtensionID
	agent.Tier = req.Tier
	agent.Position = req.Position
	agent.State = req.State
	agent.Status = "Waiting"
	agent.Active = req.Active

	err := db.QueryRowContext(ctx, query,
		req.QueueID, req.ExtensionID, req.Tier, req.Position, req.State, req.Active,
	).Scan(&agent.ID, &agent.CreatedAt, &agent.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert queue agent: %w", err)
	}

	return db.GetQueueAgent(ctx, agent.ID)
}

// UpdateQueueAgent updates a queue agent
func (db *DB) UpdateQueueAgent(ctx context.Context, id int64, req *models.QueueAgentUpdateRequest) (*models.QueueAgent, error) {
	var setClauses []string
	var args []interface{}
	argPos := 1

	if req.Tier != nil {
		setClauses = append(setClauses, fmt.Sprintf("tier = $%d", argPos))
		args = append(args, *req.Tier)
		argPos++
	}

	if req.Position != nil {
		setClauses = append(setClauses, fmt.Sprintf("position = $%d", argPos))
		args = append(args, *req.Position)
		argPos++
	}

	if req.State != nil {
		setClauses = append(setClauses, fmt.Sprintf("state = $%d", argPos))
		args = append(args, *req.State)
		argPos++
	}

	if req.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argPos))
		args = append(args, *req.Status)
		argPos++
	}

	if req.Active != nil {
		setClauses = append(setClauses, fmt.Sprintf("active = $%d", argPos))
		args = append(args, *req.Active)
		argPos++
	}

	if len(setClauses) == 0 {
		return db.GetQueueAgent(ctx, id)
	}

	// Add updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	// Add ID for WHERE clause
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE voip.queue_agents
		SET %s
		WHERE id = $%d
	`, strings.Join(setClauses, ", "), argPos)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update queue agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("queue agent not found: %d", id)
	}

	return db.GetQueueAgent(ctx, id)
}

// DeleteQueueAgent removes an agent from a queue
func (db *DB) DeleteQueueAgent(ctx context.Context, id int64) error {
	query := `
		UPDATE voip.queue_agents
		SET active = false, updated_at = $1
		WHERE id = $2
	`

	result, err := db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("delete queue agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("queue agent not found: %d", id)
	}

	return nil
}
