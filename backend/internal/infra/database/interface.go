package database

import (
	"context"

	"gorm.io/gorm"
)

// DB is the interface for database operations.
// This interface allows for easy mocking in tests.
type DB interface {
	// Transaction management
	Transaction(fc func(tx DB) error) error
	WithContext(ctx context.Context) DB

	// Basic CRUD
	Create(value interface{}) error
	First(dest interface{}, conds ...interface{}) error
	Find(dest interface{}, conds ...interface{}) error
	Save(value interface{}) error
	Delete(value interface{}, conds ...interface{}) error
	Updates(model interface{}, values interface{}) error

	// Query building
	Model(value interface{}) DB
	Table(name string) DB
	Where(query interface{}, args ...interface{}) DB
	Select(query interface{}, args ...interface{}) DB
	Joins(query string, args ...interface{}) DB
	Preload(query string, args ...interface{}) DB
	Order(value interface{}) DB
	Limit(limit int) DB
	Offset(offset int) DB
	Group(name string) DB
	Count(count *int64) error
	Scan(dest interface{}) error

	// Get underlying GORM DB (for complex queries)
	GormDB() *gorm.DB
}

// GormWrapper wraps *gorm.DB to implement DB interface
type GormWrapper struct {
	db *gorm.DB
}

// NewGormWrapper creates a new GormWrapper
func NewGormWrapper(db *gorm.DB) *GormWrapper {
	return &GormWrapper{db: db}
}

// Transaction implements DB interface
func (w *GormWrapper) Transaction(fc func(tx DB) error) error {
	return w.db.Transaction(func(tx *gorm.DB) error {
		return fc(&GormWrapper{db: tx})
	})
}

// WithContext implements DB interface
func (w *GormWrapper) WithContext(ctx context.Context) DB {
	return &GormWrapper{db: w.db.WithContext(ctx)}
}

// Create implements DB interface
func (w *GormWrapper) Create(value interface{}) error {
	return w.db.Create(value).Error
}

// First implements DB interface
func (w *GormWrapper) First(dest interface{}, conds ...interface{}) error {
	return w.db.First(dest, conds...).Error
}

// Find implements DB interface
func (w *GormWrapper) Find(dest interface{}, conds ...interface{}) error {
	return w.db.Find(dest, conds...).Error
}

// Save implements DB interface
func (w *GormWrapper) Save(value interface{}) error {
	return w.db.Save(value).Error
}

// Delete implements DB interface
func (w *GormWrapper) Delete(value interface{}, conds ...interface{}) error {
	return w.db.Delete(value, conds...).Error
}

// Updates implements DB interface
func (w *GormWrapper) Updates(model interface{}, values interface{}) error {
	return w.db.Model(model).Updates(values).Error
}

// Model implements DB interface
func (w *GormWrapper) Model(value interface{}) DB {
	return &GormWrapper{db: w.db.Model(value)}
}

// Table implements DB interface
func (w *GormWrapper) Table(name string) DB {
	return &GormWrapper{db: w.db.Table(name)}
}

// Where implements DB interface
func (w *GormWrapper) Where(query interface{}, args ...interface{}) DB {
	return &GormWrapper{db: w.db.Where(query, args...)}
}

// Select implements DB interface
func (w *GormWrapper) Select(query interface{}, args ...interface{}) DB {
	return &GormWrapper{db: w.db.Select(query, args...)}
}

// Joins implements DB interface
func (w *GormWrapper) Joins(query string, args ...interface{}) DB {
	return &GormWrapper{db: w.db.Joins(query, args...)}
}

// Preload implements DB interface
func (w *GormWrapper) Preload(query string, args ...interface{}) DB {
	return &GormWrapper{db: w.db.Preload(query, args...)}
}

// Order implements DB interface
func (w *GormWrapper) Order(value interface{}) DB {
	return &GormWrapper{db: w.db.Order(value)}
}

// Limit implements DB interface
func (w *GormWrapper) Limit(limit int) DB {
	return &GormWrapper{db: w.db.Limit(limit)}
}

// Offset implements DB interface
func (w *GormWrapper) Offset(offset int) DB {
	return &GormWrapper{db: w.db.Offset(offset)}
}

// Group implements DB interface
func (w *GormWrapper) Group(name string) DB {
	return &GormWrapper{db: w.db.Group(name)}
}

// Count implements DB interface
func (w *GormWrapper) Count(count *int64) error {
	return w.db.Count(count).Error
}

// Scan implements DB interface
func (w *GormWrapper) Scan(dest interface{}) error {
	return w.db.Scan(dest).Error
}

// GormDB returns the underlying *gorm.DB
func (w *GormWrapper) GormDB() *gorm.DB {
	return w.db
}

// Ensure GormWrapper implements DB
var _ DB = (*GormWrapper)(nil)
