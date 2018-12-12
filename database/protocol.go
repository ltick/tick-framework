package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/juju/errors"
	"github.com/ltick/tick-framework/config"
	"github.com/tsuna/gohbase/hrpc"
)

var (
	errPrepare       = "database: prepare '%s' error"
	errInitiate      = "database: initiate '%s' error"
	errStartup       = "database: startup '%s' error"
	errRegister      = "database: register error"
	errNosqlRegister = "database: register error"
	errNosqlUse      = "database: register error"
	errNewHandler    = "database: new '%s' handler error"
	errGetHandler    = "database: get '%s' handler error"
)

func NewDatabase() *Database {
	instance := &Database{}
	return instance
}

type Database struct {
	Config        *config.Config `inject:"true"`
	configs       map[string]interface{}
	provider      string
	handler       Handler
	nosqlProvider string
	nosqlHandler  NosqlHandler
}

func (d *Database) Prepare(ctx context.Context) (context.Context, error) {
	var configs map[string]config.Option = map[string]config.Option{
		"DATABASE_PROVIDER":             config.Option{Type: config.String, EnvironmentKey: "DATABASE_PROVIDER"},
		"DATABASE_MYSQL_HOST":           config.Option{Type: config.String, EnvironmentKey: "DATABASE_MYSQL_HOST"},
		"DATABASE_MYSQL_PORT":           config.Option{Type: config.String, EnvironmentKey: "DATABASE_MYSQL_PORT"},
		"DATABASE_MYSQL_USER":           config.Option{Type: config.String, EnvironmentKey: "DATABASE_MYSQL_USER"},
		"DATABASE_MYSQL_PASSWORD":       config.Option{Type: config.String, EnvironmentKey: "DATABASE_MYSQL_PASSWORD"},
		"DATABASE_MYSQL_DATABASE":       config.Option{Type: config.String, EnvironmentKey: "DATABASE_MYSQL_DATABASE"},
		"DATABASE_MYSQL_TIMEOUT":        config.Option{Type: config.String, EnvironmentKey: "DATABASE_MYSQL_TIMEOUT"},
		"DATABASE_MYSQL_MAX_OPEN_CONNS": config.Option{Type: config.Int, EnvironmentKey: "DATABASE_MYSQL_MAX_OPEN_CONNS"},
		"DATABASE_MYSQL_MAX_IDLE_CONNS": config.Option{Type: config.Int, EnvironmentKey: "DATABASE_MYSQL_MAX_IDLE_CONNS"},

		"DATABASE_NOSQL_PROVIDER":   config.Option{Type: config.String, EnvironmentKey: "DATABASE_PROVIDER"},
		"DATABASE_HBASE_HOST":       config.Option{Type: config.String, EnvironmentKey: "DATABASE_HBASE_HOST"},
		"DATABASE_HBASE_TIMEOUT":    config.Option{Type: config.String, EnvironmentKey: "DATABASE_HBASE_TIMEOUT"},
		"DATABASE_HBASE_MAX_ACTIVE": config.Option{Type: config.Int, EnvironmentKey: "DATABASE_HBASE_MAX_ACTIVE"},
	}
	err := d.Config.SetOptions(configs)
	if err != nil {
		return ctx, fmt.Errorf(errPrepare+": %s", err.Error())
	}
	return ctx, nil
}

func (d *Database) Initiate(ctx context.Context) (context.Context, error) {
	err := Register("mysql", NewMysqlHandler)
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, d.provider))
	}
	err = d.Use(ctx, "mysql")
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, d.provider))
	}
	err = NosqlRegister("hbase", NewHbaseHandler)
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, d.nosqlProvider))
	}
	err = d.NosqlUse(ctx, "hbase")
	if err != nil {
		return ctx, errors.Annotate(err, fmt.Sprintf(errInitiate, d.nosqlProvider))
	}
	d.configs = make(map[string]interface{})
	if _, ok := d.configs["DATABASE_MYSQL_HOST"]; !ok {
		d.configs["DATABASE_MYSQL_HOST"] = d.Config.GetString("DATABASE_MYSQL_HOST")
	}
	if _, ok := d.configs["DATABASE_MYSQL_PORT"]; !ok {
		d.configs["DATABASE_MYSQL_PORT"] = d.Config.GetString("DATABASE_MYSQL_PORT")
	}
	if _, ok := d.configs["DATABASE_MYSQL_USER"]; !ok {
		d.configs["DATABASE_MYSQL_USER"] = d.Config.GetString("DATABASE_MYSQL_USER")
	}
	if _, ok := d.configs["DATABASE_MYSQL_PASSWORD"]; !ok {
		d.configs["DATABASE_MYSQL_PASSWORD"] = d.Config.GetString("DATABASE_MYSQL_PASSWORD")
	}
	if _, ok := d.configs["DATABASE_MYSQL_DATABASE"]; !ok {
		d.configs["DATABASE_MYSQL_DATABASE"] = d.Config.GetString("DATABASE_MYSQL_DATABASE")
	}
	if _, ok := d.configs["DATABASE_MYSQL_TIMEZONE"]; !ok {
		d.configs["DATABASE_MYSQL_TIMEZONE"] = d.Config.GetString("DATABASE_MYSQL_TIMEZONE")
	}
	if _, ok := d.configs["DATABASE_MYSQL_TIMEOUT"]; !ok {
		d.configs["DATABASE_MYSQL_TIMEOUT"] = d.Config.GetString("DATABASE_MYSQL_TIMEOUT")
	}
	if _, ok := d.configs["DATABASE_MYSQL_WRITE_TIMEOUT"]; !ok {
		d.configs["DATABASE_MYSQL_WRITE_TIMEOUT"] = d.Config.GetString("DATABASE_MYSQL_WRITE_TIMEOUT")
	}
	if _, ok := d.configs["DATABASE_MYSQL_READ_TIMEOUT"]; !ok {
		d.configs["DATABASE_MYSQL_READ_TIMEOUT"] = d.Config.GetString("DATABASE_MYSQL_READ_TIMEOUT")
	}
	if _, ok := d.configs["DATABASE_MYSQL_MAX_OPEN_CONNS"]; !ok {
		d.configs["DATABASE_MYSQL_MAX_OPEN_CONNS"] = d.Config.GetInt("DATABASE_MYSQL_MAX_OPEN_CONNS")
	}
	if _, ok := d.configs["DATABASE_MYSQL_MAX_IDLE_CONNS"]; !ok {
		d.configs["DATABASE_MYSQL_MAX_IDLE_CONNS"] = d.Config.GetInt("DATABASE_MYSQL_MAX_IDLE_CONNS")
	}
	return ctx, nil
}
func (d *Database) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	databaseProvider := d.Config.GetString("DATABASE_PROVIDER")
	if databaseProvider != "" {
		err = d.Use(ctx, databaseProvider)
		if err != nil {
			return ctx, errors.Annotate(err, fmt.Sprintf(errStartup+": "+err.Error(), d.provider))
		}
	}
	databaseNosqlProvider := d.Config.GetString("DATABASE_NOSQL_PROVIDER")
	if databaseNosqlProvider != "" {
		err = d.NosqlUse(ctx, databaseNosqlProvider)
		if err != nil {
			return ctx, errors.Annotate(err, fmt.Sprintf(errStartup+": "+err.Error(), d.nosqlProvider))
		}
	}
	return ctx, nil
}
func (d *Database) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (d *Database) GetProvider() string {
	return d.provider
}
func (d *Database) Use(ctx context.Context, Provider string) error {
	handler, err := Use(Provider)
	if err != nil {
		return err
	}
	d.provider = Provider
	d.handler = handler()
	err = d.handler.Initiate(ctx)
	if err != nil {
		return errors.Annotate(err, fmt.Sprintf(errInitiate, d.provider))
	}
	return nil
}
func (d *Database) NewHandler(name string, config map[string]interface{}) (DatabaseHandler, error) {
	databaseHandler, err := d.handler.GetHandler(name)
	if err == nil {
		return databaseHandler, nil
	}
	databaseHandler, err = d.handler.NewHandler(name, d.configs)
	if err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errNewHandler, name))
	}
	if databaseHandler == nil {
		return nil, errors.Annotate(errors.New("database: empty database"), fmt.Sprintf(errNewHandler, name))
	}
	return databaseHandler, nil
}
func (d *Database) GetHandler(name string) (DatabaseHandler, error) {
	databaseHandler, err := d.handler.GetHandler(name)
	if err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errGetHandler, name))
	}
	return databaseHandler, err
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewHandler(name string, config map[string]interface{}) (DatabaseHandler, error)
	GetHandler(name string) (DatabaseHandler, error)
}

type DatabaseCallback interface {
	Create() *gorm.CallbackProcessor
	Update() *gorm.CallbackProcessor
	Delete() *gorm.CallbackProcessor
	Query() *gorm.CallbackProcessor
	RowQuery() *gorm.CallbackProcessor
}
type DatabaseHandler interface {
	GetConfig() map[string]interface{}
	New() DatabaseHandler
	Close() error
	Model(value interface{}) DatabaseHandler
	Table(name string) DatabaseHandler
	Debug() DatabaseHandler
	Error() error
	Callback() DatabaseCallback
	// NewRecord check if value's primary key is blank
	NewRecord(value interface{}) bool
	// RecordNotFound check if returning error
	RecordNotFound() bool
	//Table
	CreateTable(models ...interface{}) DatabaseHandler
	Set(name string, value interface{}) DatabaseHandler
	AutoMigrate(values ...interface{}) DatabaseHandler
	DropTable(values ...interface{}) DatabaseHandler
	DropTableIfExists(values ...interface{}) DatabaseHandler
	HasTable(value interface{}) bool
	ModifyColumn(column string, typ string) DatabaseHandler
	DropColumn(column string) DatabaseHandler
	AddIndex(indexName string, columns ...string) DatabaseHandler
	AddUniqueIndex(indexName string, columns ...string) DatabaseHandler
	RemoveIndex(indexName string) DatabaseHandler
	AddForeignKey(field string, dest string, onDelete string, onUpdate string) DatabaseHandler
	// Query
	Where(query interface{}, args ...interface{}) DatabaseHandler
	Or(query interface{}, args ...interface{}) DatabaseHandler
	Not(query interface{}, args ...interface{}) DatabaseHandler
	Limit(limit interface{}) DatabaseHandler
	Offset(offset interface{}) DatabaseHandler
	Order(value interface{}, reorder ...bool) DatabaseHandler
	Select(query interface{}, args ...interface{}) DatabaseHandler
	Omit(columns ...string) DatabaseHandler
	Having(query string, values ...interface{}) DatabaseHandler
	Joins(query string, args ...interface{}) DatabaseHandler
	Find(out interface{}, where ...interface{}) DatabaseHandler
	First(out interface{}, where ...interface{}) DatabaseHandler
	Last(out interface{}, where ...interface{}) DatabaseHandler
	Row() *sql.Row
	Rows() (*sql.Rows, error)
	Pluck(column string, value interface{}) DatabaseHandler
	Count(value interface{}) DatabaseHandler
	Related(value interface{}, foreignKeys ...string) DatabaseHandler
	Scan(dest interface{}) DatabaseHandler
	// Update
	Update(attrs ...interface{}) DatabaseHandler
	Updates(values interface{}, ignoreProtectedAttrs ...bool) DatabaseHandler
	UpdateColumn(attrs ...interface{}) DatabaseHandler
	UpdateColumns(values interface{}) DatabaseHandler
	Save(value interface{}) DatabaseHandler
	// Insert
	Create(value interface{}) DatabaseHandler
	// Delete
	Delete(value interface{}, where ...interface{}) DatabaseHandler
	Unscoped() DatabaseHandler
	Scopes(funcs ...func(*gorm.DB) *gorm.DB) DatabaseHandler
	// Raw Sql
	Raw(sql string, values ...interface{}) DatabaseHandler
	Exec(sql string, values ...interface{}) DatabaseHandler
	// Transaction
	Begin() DatabaseHandler
	Commit() DatabaseHandler
	Rollback() DatabaseHandler
}

type databaseHandler func() Handler

var databaseHandlers = make(map[string]databaseHandler)

func Register(name string, databaseHandler databaseHandler) error {
	if databaseHandler == nil {
		return errors.Annotate(errors.New("database: Register database is nil"), errRegister)
	}
	if _, ok := databaseHandlers[name]; !ok {
		databaseHandlers[name] = databaseHandler
	}
	return nil
}
func Use(name string) (databaseHandler, error) {
	if _, exist := databaseHandlers[name]; !exist {
		return nil, errors.Annotate(errors.Errorf("database: unknown database '%s' (forgotten register?)", name), errRegister)
	}
	return databaseHandlers[name], nil
}

/****************** Nosql ******************/
func (d *Database) NosqlUse(ctx context.Context, Provider string) error {
	nosqlHandler, err := NosqlUse(Provider)
	if err != nil {
		return err
	}
	d.nosqlProvider = Provider
	d.nosqlHandler = nosqlHandler()
	err = d.nosqlHandler.Initiate(ctx)
	if err != nil {
		return errors.Annotate(err, fmt.Sprintf(errInitiate, d.provider))
	}
	return nil
}
func (d *Database) NewNosqlHandler(name string, config map[string]interface{}) (NosqlDatabaseHandler, error) {
	database, err := d.GetNosqlHandler(name)
	if err == nil {
		return database, nil
	}
	if _, ok := config["DATABASE_HBASE_HOST"]; !ok {
		config["DATABASE_HBASE_HOST"] = d.Config.GetString("DATABASE_HBASE_HOST")
	}
	if _, ok := config["DATABASE_HBASE_PORT"]; !ok {
		config["DATABASE_HBASE_PORT"] = d.Config.GetString("DATABASE_HBASE_PORT")
	}
	if _, ok := config["DATABASE_HBASE_USER"]; !ok {
		config["DATABASE_HBASE_USER"] = d.Config.GetString("DATABASE_HBASE_USER")
	}
	if _, ok := config["DATABASE_HBASE_PASSWORD"]; !ok {
		config["DATABASE_HBASE_PASSWORD"] = d.Config.GetString("DATABASE_HBASE_PASSWORD")
	}
	if _, ok := config["DATABASE_HBASE_DATABASE"]; !ok {
		config["DATABASE_HBASE_DATABASE"] = d.Config.GetString("DATABASE_HBASE_DATABASE")
	}
	if _, ok := config["DATABASE_HBASE_TIMEOUT"]; !ok {
		config["DATABASE_HBASE_TIMEOUT"] = d.Config.GetString("DATABASE_HBASE_TIMEOUT")
	}
	if _, ok := config["DATABASE_HBASE_MAX_ACTIVE"]; !ok {
		config["DATABASE_HBASE_MAX_ACTIVE"] = d.Config.GetInt("DATABASE_HBASE_MAX_ACTIVE")
	}
	database, err = d.nosqlHandler.NewHandler(name, config)
	if err != nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errNewHandler, name))
	}
	if database == nil {
		return nil, errors.Annotate(err, fmt.Sprintf(errNewHandler+": empty database", name))
	}
	return database, nil
}
func (d *Database) GetNosqlHandler(name string) (NosqlDatabaseHandler, error) {
	databaseHandler, err := d.nosqlHandler.GetHandler(name)
	if err != nil {
		if HandlerNotExists(err) {
			databaseHandler, err = d.nosqlHandler.NewHandler(name, d.configs)
		}
		return nil, errors.Annotate(err, fmt.Sprintf(errGetHandler, name))
	}
	return databaseHandler, err
}

type NosqlHandler interface {
	Initiate(ctx context.Context) error
	NewHandler(name string, config map[string]interface{}) (NosqlDatabaseHandler, error)
	GetHandler(name string) (NosqlDatabaseHandler, error)
}
type NosqlDatabaseCallback interface {
}
type NosqlDatabaseScanner interface {
	Next() (*hrpc.Result, error)
	Close() error
}
type NosqlDatabaseHandler interface {
	GetHandler() (client interface{}, err error)
	ReleaseHandler(client interface{})
	GetHandlerPoolSize() int
	Scan(ctx context.Context, table string) (NosqlDatabaseScanner, error)
	Get(ctx context.Context, table string, key string) ([]map[string]string, error)
	Put(ctx context.Context, table string, key string, values map[string]map[string][]byte) (err error)
	Delete(ctx context.Context, table string, key string, values map[string]map[string][]byte) (err error)
	Append(ctx context.Context, table string, key string, values map[string]map[string][]byte) error
	Increment(ctx context.Context, table string, key string, values map[string]map[string][]byte) (int64, error)
	CheckAndPut(ctx context.Context, table string, key string, values map[string]map[string][]byte, family string, qualifier string, expectedValue []byte) (bool, error)
	Close()
}

type nosqlDatabaseHandler func() NosqlHandler

var nosqlDatabaseHandlers = make(map[string]nosqlDatabaseHandler)

func NosqlRegister(name string, nosqlDatabaseHandler nosqlDatabaseHandler) error {
	if nosqlDatabaseHandler == nil {
		return errors.Annotate(errors.New("database: Register nosql database is nil"), errNosqlRegister)
	}
	if _, ok := nosqlDatabaseHandlers[name]; !ok {
		nosqlDatabaseHandlers[name] = nosqlDatabaseHandler
	}
	return nil
}

func NosqlUse(name string) (nosqlDatabaseHandler, error) {
	if _, dup := nosqlDatabaseHandlers[name]; !dup {
		return nil, errors.Annotate(errors.Errorf("database: unknown nosql database '%s' (forgotten register?)", name), errNosqlUse)
	}
	return nosqlDatabaseHandlers[name], nil
}

func HandlerNotExists(err error) bool {
	return strings.Contains(err.Error(), "handler not exists")
}
