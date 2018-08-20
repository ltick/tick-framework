package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/ltick/tick-framework/module/config"
	"github.com/ltick/tick-framework/module/utility"
	"github.com/ltick/tick-routing"
	"github.com/tsuna/gohbase/hrpc"
)

var (
	errInitiate    = "database: initiate '%s' error"
	errStartup     = "database: startup '%s' error"
	errNewDatabase = "database: new '%s' database error"
	errGetDatabase = "database: get '%s' database error"
)

func NewInstance() *Instance {
	instance := &Instance{
		Utility: &utility.Instance{},
	}
	return instance
}

type Instance struct {
	Config           *config.Instance
	Utility          *utility.Instance
	handlerName      string
	handler          Handler
	nosqlHandlerName string
	nosqlHandler     NosqlHandler
}

func (this *Instance) Initiate(ctx context.Context) (newCtx context.Context, err error) {
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

		"DATABASE_HBASE_HOST":       config.Option{Type: config.String, EnvironmentKey: "DATABASE_HBASE_HOST"},
		"DATABASE_HBASE_TIMEOUT":    config.Option{Type: config.String, EnvironmentKey: "DATABASE_HBASE_TIMEOUT"},
		"DATABASE_HBASE_MAX_ACTIVE": config.Option{Type: config.Int, EnvironmentKey: "DATABASE_HBASE_MAX_ACTIVE"},
	}
	newCtx, err = this.Config.SetOptions(ctx, configs)
	if err != nil {
		return newCtx, fmt.Errorf(errInitiate+": %s", err.Error())
	}
	err = Register("mysql", NewMysqlHandler)
	if err != nil {
		return newCtx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	err = this.Use(newCtx, "mysql")
	if err != nil {
		return newCtx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	err = NosqlRegister("hbase", NewHbaseHandler)
	if err != nil {
		return newCtx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	err = this.NosqlUse(newCtx, "hbase")
	if err != nil {
		return newCtx, errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return newCtx, nil
}
func (this *Instance) OnStartup(ctx context.Context) (context.Context, error) {
	var err error
	databaseProvider := this.Config.GetString("DATABASE_PROVIDER")
	if databaseProvider != "" {
		err = this.Use(ctx, databaseProvider)
	} else {
		err = this.Use(ctx, "mysql")
	}
	if err != nil {
		return ctx, errors.New(fmt.Sprintf(errStartup+": "+err.Error(), this.handlerName))
	}
	return ctx, nil
}
func (this *Instance) OnShutdown(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
func (this *Instance) OnRequestStartup(c *routing.Context) error {
	return nil
}
func (this *Instance) OnRequestShutdown(c *routing.Context) error {
	return nil
}
func (this *Instance) HandlerName() string {
	return this.handlerName
}
func (this *Instance) Use(ctx context.Context, handlerName string) error {
	handler, err := Use(handlerName)
	if err != nil {
		return err
	}
	this.handlerName = handlerName
	this.handler = handler()
	err = this.handler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return nil
}
func (this *Instance) NewDatabase(ctx context.Context, name string) (DatabaseHandler, error) {
	database, err := this.GetDatabase(name)
	if err == nil {
		return database, nil
	}
	config := map[string]interface{}{
		"DATABASE_MYSQL_HOST":           this.Config.GetString("DATABASE_MYSQL_HOST"),
		"DATABASE_MYSQL_PORT":           this.Config.GetString("DATABASE_MYSQL_PORT"),
		"DATABASE_MYSQL_USER":           this.Config.GetString("DATABASE_MYSQL_USER"),
		"DATABASE_MYSQL_PASSWORD":       this.Config.GetString("DATABASE_MYSQL_PASSWORD"),
		"DATABASE_MYSQL_DATABASE":       this.Config.GetString("DATABASE_MYSQL_DATABASE"),
		"DATABASE_MYSQL_TIMEZONE":       this.Config.GetString("DATABASE_MYSQL_TIMEZONE"),
		"DATABASE_MYSQL_TIMEOUT":        this.Config.GetString("DATABASE_MYSQL_TIMEOUT"),
		"DATABASE_MYSQL_WRITE_TIMEOUT":  this.Config.GetString("DATABASE_MYSQL_WRITE_TIMEOUT"),
		"DATABASE_MYSQL_READ_TIMEOUT":   this.Config.GetString("DATABASE_MYSQL_READ_TIMEOUT"),
		"DATABASE_MYSQL_MAX_OPEN_CONNS": this.Config.GetInt("DATABASE_MYSQL_MAX_OPEN_CONNS"),
		"DATABASE_MYSQL_MAX_IDLE_CONNS": this.Config.GetInt("DATABASE_MYSQL_MAX_IDLE_CONNS"),
	}
	database, err = this.handler.NewDatabase(ctx, name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewDatabase+": "+err.Error(), name))
	}
	if database == nil {
		return nil, errors.New(fmt.Sprintf(errNewDatabase+": empty database", name))
	}

	return database, nil
}
func (this *Instance) GetDatabase(name string) (DatabaseHandler, error) {
	handlerDatabase, err := this.handler.GetDatabase(name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errGetDatabase+": "+err.Error(), name))
	}
	return handlerDatabase, err
}

type Handler interface {
	Initiate(ctx context.Context) error
	NewDatabase(ctx context.Context, name string, config map[string]interface{}) (DatabaseHandler, error)
	GetDatabase(name string) (DatabaseHandler, error)
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
		return errors.New("database: Register database is nil")
	}
	if _, ok := databaseHandlers[name]; !ok {
		databaseHandlers[name] = databaseHandler
	}
	return nil
}
func Use(name string) (databaseHandler, error) {
	if _, exist := databaseHandlers[name]; !exist {
		return nil, errors.New(fmt.Sprintf("database: unknown database '%s' (forgotten register?)", name))
	}
	return databaseHandlers[name], nil
}

/****************** Nosql ******************/
func (this *Instance) NosqlUse(ctx context.Context, handlerName string) error {
	nosqlHandler, err := NosqlUse(handlerName)
	if err != nil {
		return err
	}
	this.nosqlHandlerName = handlerName
	this.nosqlHandler = nosqlHandler()
	err = this.nosqlHandler.Initiate(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf(errInitiate+": "+err.Error(), this.handlerName))
	}
	return nil
}
func (this *Instance) NewNosqlDatabase(ctx context.Context, name string) (NosqlDatabaseHandler, error) {
	database, err := this.GetNosqlDatabase(name)
	if err == nil {
		return database, nil
	}
	config := map[string]interface{}{
		"DATABASE_HBASE_HOST":       this.Config.GetString("DATABASE_HBASE_HOST"),
		"DATABASE_HBASE_PORT":       this.Config.GetString("DATABASE_HBASE_PORT"),
		"DATABASE_HBASE_USER":       this.Config.GetString("DATABASE_HBASE_USER"),
		"DATABASE_HBASE_PASSWORD":   this.Config.GetString("DATABASE_HBASE_PASSWORD"),
		"DATABASE_HBASE_DATABASE":   this.Config.GetString("DATABASE_HBASE_DATABASE"),
		"DATABASE_HBASE_TIMEOUT":    this.Config.GetString("DATABASE_HBASE_TIMEOUT"),
		"DATABASE_HBASE_MAX_ACTIVE": this.Config.GetInt("DATABASE_HBASE_MAX_ACTIVE"),
	}
	database, err = this.nosqlHandler.NewDatabase(ctx, name, config)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errNewDatabase+": "+err.Error(), name))
	}
	if database == nil {
		return nil, errors.New(fmt.Sprintf(errNewDatabase+": empty database", name))
	}
	return database, nil
}
func (this *Instance) GetNosqlDatabase(name string) (NosqlDatabaseHandler, error) {
	handlerDatabase, err := this.nosqlHandler.GetDatabase(name)
	if err != nil {
		return nil, errors.New(fmt.Sprintf(errGetDatabase+": "+err.Error(), name))
	}
	return handlerDatabase, err
}

type NosqlHandler interface {
	Initiate(ctx context.Context) error
	NewDatabase(ctx context.Context, name string, config map[string]interface{}) (NosqlDatabaseHandler, error)
	GetDatabase(name string) (NosqlDatabaseHandler, error)
}
type NosqlDatabaseCallback interface {
}
type NosqlDatabaseScanner interface {
	Next() (*hrpc.Result, error)
	Close() error
}
type NosqlDatabaseHandler interface {
	GetConnection() (client interface{}, err error)
	ReleaseConnection(client interface{})
	GetConnectionPoolSize() int
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
		return errors.New("database: Register nosql database is nil")
	}
	if _, ok := nosqlDatabaseHandlers[name]; !ok {
		nosqlDatabaseHandlers[name] = nosqlDatabaseHandler
	}
	return nil
}

func NosqlUse(name string) (nosqlDatabaseHandler, error) {
	if _, dup := nosqlDatabaseHandlers[name]; !dup {
		return nil, errors.New(fmt.Sprintf("database: unknown nosql database '%s' (forgotten register?)", name))
	}

	return nosqlDatabaseHandlers[name], nil
}
