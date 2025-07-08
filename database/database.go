package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gormLogger adapts slog to be used as a GORM logger.
type gormLogger struct {
	slogger *slog.Logger
}

// Implement the GORM logger interface
func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	// We can choose to return a new logger with a different level, but for now, we'll reuse the same.
	return l
}
func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	l.slogger.InfoContext(ctx, msg, "gorm_data", data)
}
func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.slogger.WarnContext(ctx, msg, "gorm_data", data)
}
func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.slogger.ErrorContext(ctx, msg, "gorm_data", data)
}
func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	attrs := []slog.Attr{
		slog.String("latency", elapsed.String()),
		slog.String("sql", sql),
		slog.Int64("rows_affected", rows),
	}

	// Use slog.LogAttrs which is designed to handle a slice of Attrs.
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		attrs = append(attrs, slog.Any("error", err))
		l.slogger.LogAttrs(ctx, slog.LevelError, "GORM Trace", attrs...)
	} else {
		l.slogger.LogAttrs(ctx, slog.LevelDebug, "GORM Trace", attrs...)
	}
}

// Database holds the DB connection, all repository instances, and the UnitOfWork.
type Database struct {
	DB                 *gorm.DB
	UoW                UnitOfWorkInterface
	ConnectionRepo     interfaces.ConnectionRepositoryInterface
	FactsheetRepo      interfaces.FactsheetRepositoryInterface
	ActionRepo         interfaces.ActionRepositoryInterface
	NodeRepo           interfaces.NodeRepositoryInterface
	EdgeRepo           interfaces.EdgeRepositoryInterface
	OrderTemplateRepo  interfaces.OrderTemplateRepositoryInterface
	OrderExecutionRepo interfaces.OrderExecutionRepositoryInterface
}

// NewDatabase creates a new database connection and initializes repositories.
func NewDatabase(cfg *config.Config, appLogger *slog.Logger) (*Database, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Seoul",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)

	dbLogger := appLogger.With("component", "database")
	dbLogger.Info("Connecting to database...", "host", cfg.DBHost, "port", cfg.DBPort, "user", cfg.DBUser)

	// Configure GORM to use our structured logger
	newGormLogger := &gormLogger{slogger: dbLogger}
	gormConfig := &gorm.Config{
		Logger: newGormLogger.LogMode(logger.Info), // Set the desired GORM log level
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	dbLogger.Info("Database connected successfully")

	dbLogger.Info("Starting database migration...")
	err = db.AutoMigrate(
		&models.ConnectionState{}, &models.ConnectionStateHistory{},
		&models.AgvAction{}, &models.AgvActionParameter{},
		&models.PhysicalParameter{}, &models.TypeSpecification{},
		&models.ActionTemplate{}, &models.ActionParameterTemplate{},
		&models.OrderTemplate{}, &models.OrderExecution{},
		&models.NodeTemplate{}, &models.EdgeTemplate{},
		&models.OrderTemplateNode{}, &models.OrderTemplateEdge{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	dbLogger.Info("Database migration completed successfully")

	// Initialize repositories
	connectionRepo := repositories.NewConnectionRepository(db)
	factsheetRepo := repositories.NewFactsheetRepository(db)
	actionRepo := repositories.NewActionRepository(db)
	nodeRepo := repositories.NewNodeRepository(db)
	edgeRepo := repositories.NewEdgeRepository(db)
	orderTemplateRepo := repositories.NewOrderTemplateRepository(db)
	orderExecutionRepo := repositories.NewOrderExecutionRepository(db)

	return &Database{
		DB:                 db,
		UoW:                NewUnitOfWork(db),
		ConnectionRepo:     connectionRepo,
		FactsheetRepo:      factsheetRepo,
		ActionRepo:         actionRepo,
		NodeRepo:           nodeRepo,
		EdgeRepo:           edgeRepo,
		OrderTemplateRepo:  orderTemplateRepo,
		OrderExecutionRepo: orderExecutionRepo,
	}, nil
}

// Legacy methods are kept for backward compatibility if needed elsewhere,
// though direct repository usage is preferred.
func (d *Database) SaveConnectionState(tx *gorm.DB, connectionMsg *models.ConnectionMessage) error {
	return d.ConnectionRepo.SaveConnectionState(tx, connectionMsg)
}

func (d *Database) SaveOrUpdateFactsheet(tx *gorm.DB, factsheetMsg *models.FactsheetMessage) error {
	return d.FactsheetRepo.SaveOrUpdateFactsheet(tx, factsheetMsg)
}

func (d *Database) GetLastConnectionState(serialNumber string) (*models.ConnectionState, error) {
	return d.ConnectionRepo.GetLastConnectionState(serialNumber)
}

func (d *Database) DebugAgvActions(serialNumber string) {
	d.FactsheetRepo.DebugAgvActions(serialNumber)
}
