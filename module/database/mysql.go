package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

var (
	errMysqlRegister         = "database(mysql): initiate error"
	errMysqlNewDatabase      = "database(mysql): new database error"
	errMysqlDatabseNotExists = "database(mysql): database '%s' not exists"
)

type MysqlHandler struct {
	databases map[string]*MysqlDatabaseHandler
}

func NewMysqlHandler() Handler {
	return &MysqlHandler{}
}

func (this *MysqlHandler) Initiate(ctx context.Context) error {
	this.databases = make(map[string]*MysqlDatabaseHandler)
	return nil
}

func (this *MysqlHandler) NewDatabase(ctx context.Context, name string, config map[string]interface{}) (DatabaseHandler, error) {
	db := &MysqlDatabaseHandler{}
	// Default
	db.Port = "3306"
	db.Timezone = "Asia/Shanghai"
	db.Timeout = "30s"
	db.WriteTimeout = "120s"
	db.ReadTimeout = "60s"
	db.MaxOpenConns = 300
	db.MaxIdleConns = 100
	configHost := config["DATABASE_MYSQL_HOST"]
	if configHost != nil {
		host, ok := configHost.(string)
		if ok {
			db.Host = host
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_HOST")
		}
		if host == "" {
			return nil, errors.New(errMysqlNewDatabase + ": empty DATABASE_MYSQL_HOST")
		}
	} else {
		return nil, errors.New(errMysqlNewDatabase + ": empty DATABASE_MYSQL_HOST")
	}
	configPort, ok := config["DATABASE_MYSQL_PORT"]
	if ok && configPort != nil {
		port, ok := configPort.(string)
		if ok {
			db.Port = port
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_PORT")
		}
	}
	configUser, ok := config["DATABASE_MYSQL_USER"]
	if ok && configUser != nil {
		user, ok := configUser.(string)
		if ok {
			db.User = user
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_USER")
		}
		if user == "" {
			return nil, errors.New(errMysqlNewDatabase + ": empty config DATABASE_MYSQL_USER")
		}
	}
	configPassword, ok := config["DATABASE_MYSQL_PASSWORD"]
	if ok && configPassword != nil {
		password, ok := configPassword.(string)
		if ok {
			db.Password = password
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_PASSWORD")
		}
	}
	configDatabase, ok := config["DATABASE_MYSQL_DATABASE"]
	if ok && configDatabase != nil {
		database, ok := configDatabase.(string)
		if ok {
			db.Database = database
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_DATABASE")
		}
		if database == "" {
			return nil, errors.New(errMysqlNewDatabase + ": empty config DATABASE_MYSQL_DATABASE")
		}
	}
	configTimezone, ok := config["DATABASE_MYSQL_TIMEZONE"]
	if ok && configTimezone != nil {
		timezone, ok := configTimezone.(string)
		if ok  {
			if timezone != "" {
				db.Timezone = timezone
			}
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_TIMEZONE")
		}
	}
	configTimeout, ok := config["DATABASE_MYSQL_TIMEOUT"]
	if ok && configTimeout != nil {
		timeout, ok := configTimeout.(string)
		if ok && timeout != "" {
			db.Timeout = timeout
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_TIMEOUT")
		}
	}
	configWriteTimeout, ok := config["DATABASE_MYSQL_WRITE_TIMEOUT"]
	if ok && configWriteTimeout != nil {
		writeTimeout, ok := configWriteTimeout.(string)
		if ok {
			if writeTimeout != "" {
				db.WriteTimeout = writeTimeout
			}
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_WRITE_TIMEOUT")
		}
	}
	configReadTimeout, ok := config["DATABASE_MYSQL_READ_TIMEOUT"]
	if ok && configReadTimeout != nil {
		readTimeout, ok := configReadTimeout.(string)
		if ok {
			if readTimeout != "" {
				db.ReadTimeout = readTimeout
			}
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_READ_TIMEOUT")
		}
	}
	configMaxOpenConns, ok := config["DATABASE_MYSQL_MAX_OPEN_CONNS"]
	if ok && configMaxOpenConns != nil {
		maxOpenConns, ok := configMaxOpenConns.(int)
		if ok {
			if maxOpenConns != 0 {
				db.MaxOpenConns = maxOpenConns
			}
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_MAX_OPEN_CONNS")
		}
	}
	configMaxIdleConns, ok := config["DATABASE_MYSQL_MAX_IDLE_CONNS"]
	if ok && configMaxIdleConns != nil {
		maxIdleConns, ok := configMaxIdleConns.(int)
		if ok {
			if maxIdleConns != 0 {
				db.MaxIdleConns = maxIdleConns
			}
		} else {
			return nil, errors.New(errMysqlNewDatabase + ": invalid config DATABASE_MYSQL_MAX_IDLE_CONNS")
		}
	}
	args := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?timeout=%s&writeTimeout=%s&readTimeout=%s&charset=utf8mb4,utf8&loc=%s&parseTime=true",
		db.User,
		db.Password,
		db.Host,
		db.Port,
		db.Database,
		db.WriteTimeout,
		db.ReadTimeout,
		db.Timeout,
		url.QueryEscape(db.Timezone),
	)
	gormDb, err := gorm.Open("mysql", args)
	if err == nil {
		gormDb.DB().SetMaxOpenConns(db.MaxOpenConns)
		gormDb.DB().SetMaxIdleConns(db.MaxIdleConns)
		configDebug := config["DATABASE_MYSQL_DEBUG"]
		if configDebug != nil {
			if debug, ok := configDebug.(bool); ok {
				if debug {
					gormDb.Debug()
				}
			} else {
				return nil, errors.New(errMysqlNewDatabase + ": invalid DATABASE_MYSQL_DEBUG")
			}
		}
		db.Db = gormDb
		if this.databases == nil {
			this.databases = make(map[string]*MysqlDatabaseHandler)
		}
		this.databases[name] = db
	} else {
		return nil, errors.New(errMysqlNewDatabase + ": " + err.Error())
	}
	return db, nil
}

func (this *MysqlHandler) GetDatabase(name string) (DatabaseHandler, error) {
	handlerDatabase, ok := this.databases[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf(errMysqlDatabseNotExists, name))
	}
	return handlerDatabase, nil
}

type MysqlDatabaseHandler struct {
	Db           *gorm.DB
	User         string
	Password     string
	Host         string
	Port         string
	Database     string
	Timezone     string
	Timeout      string
	WriteTimeout string
	ReadTimeout  string
	MaxOpenConns int
	MaxIdleConns int
}

func (this *MysqlDatabaseHandler) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"host":           this.Host,
		"port":           this.Port,
		"user":           this.User,
		"password":       this.Password,
		"database":       this.Database,
		"timezone":       this.Timezone,
		"timeout":        this.Timeout,
		"read_timeout":   this.ReadTimeout,
		"write_timeout":  this.WriteTimeout,
		"max_open_conns": this.MaxOpenConns,
		"max_idle_conns": this.MaxIdleConns,
	}
}

func (this *MysqlDatabaseHandler) New() DatabaseHandler {
	return &MysqlDatabaseHandler{
		Db: this.Db.New(),
	}
}
func (this *MysqlDatabaseHandler) Close() error {
	return this.Db.Close()
}
func (this *MysqlDatabaseHandler) Model(value interface{}) DatabaseHandler {
	this.Db = this.Db.Model(value)
	return this
}
func (this *MysqlDatabaseHandler) Table(name string) DatabaseHandler {
	this.Db = this.Db.Table(name)
	return this
}
func (this *MysqlDatabaseHandler) Debug() DatabaseHandler {
	this.Db = this.Db.Debug()
	return this
}
func (this *MysqlDatabaseHandler) Error() error {
	return this.Db.Error
}
func (this *MysqlDatabaseHandler) Callback() DatabaseCallback {
	return this.Db.Callback()
}

func (this *MysqlDatabaseHandler) NewRecord(value interface{}) bool {
	return this.Db.NewRecord(value)
}
func (this *MysqlDatabaseHandler) RecordNotFound() bool {
	return this.Db.RecordNotFound()
}

//Table
func (this *MysqlDatabaseHandler) CreateTable(models ...interface{}) DatabaseHandler {
	this.Db = this.Db.CreateTable(models...)
	return this
}
func (this *MysqlDatabaseHandler) Set(name string, value interface{}) DatabaseHandler {
	this.Db = this.Db.Set(name, value)
	return this
}
func (this *MysqlDatabaseHandler) AutoMigrate(values ...interface{}) DatabaseHandler {
	this.Db = this.Db.AutoMigrate(values...)
	return this
}
func (this *MysqlDatabaseHandler) DropTable(values ...interface{}) DatabaseHandler {
	this.Db = this.Db.DropTable(values...)
	return this
}
func (this *MysqlDatabaseHandler) DropTableIfExists(values ...interface{}) DatabaseHandler {
	this.Db = this.Db.DropTableIfExists(values...)
	return this
}
func (this *MysqlDatabaseHandler) HasTable(value interface{}) bool {
	return this.Db.HasTable(value)
}
func (this *MysqlDatabaseHandler) ModifyColumn(column string, typ string) DatabaseHandler {
	this.Db = this.Db.ModifyColumn(column, typ)
	return this
}
func (this *MysqlDatabaseHandler) DropColumn(column string) DatabaseHandler {
	this.Db = this.Db.DropColumn(column)
	return this
}
func (this *MysqlDatabaseHandler) AddIndex(indexName string, columns ...string) DatabaseHandler {
	this.Db = this.Db.AddIndex(indexName, columns...)
	return this
}
func (this *MysqlDatabaseHandler) AddUniqueIndex(indexName string, columns ...string) DatabaseHandler {
	this.Db = this.Db.AddUniqueIndex(indexName, columns...)
	return this
}
func (this *MysqlDatabaseHandler) RemoveIndex(indexName string) DatabaseHandler {
	this.Db = this.Db.RemoveIndex(indexName)
	return this
}
func (this *MysqlDatabaseHandler) AddForeignKey(field string, dest string, onDelete string, onUpdate string) DatabaseHandler {
	this.Db = this.Db.AddForeignKey(field, dest, onDelete, onUpdate)
	return this
}

// Query
func (this *MysqlDatabaseHandler) Where(query interface{}, args ...interface{}) DatabaseHandler {
	this.Db = this.Db.Where(query, args...)
	return this
}
func (this *MysqlDatabaseHandler) Or(query interface{}, args ...interface{}) DatabaseHandler {
	this.Db = this.Db.Or(query, args...)
	return this
}
func (this *MysqlDatabaseHandler) Not(query interface{}, args ...interface{}) DatabaseHandler {
	this.Db = this.Db.Not(query, args...)
	return this
}
func (this *MysqlDatabaseHandler) Limit(limit interface{}) DatabaseHandler {
	this.Db = this.Db.Limit(limit)
	return this
}
func (this *MysqlDatabaseHandler) Offset(offset interface{}) DatabaseHandler {
	this.Db = this.Db.Offset(offset)
	return this
}
func (this *MysqlDatabaseHandler) Order(value interface{}, reorder ...bool) DatabaseHandler {
	this.Db = this.Db.Order(value, reorder...)
	return this
}
func (this *MysqlDatabaseHandler) Select(query interface{}, args ...interface{}) DatabaseHandler {
	this.Db = this.Db.Select(query, args...)
	return this
}
func (this *MysqlDatabaseHandler) Omit(columns ...string) DatabaseHandler {
	this.Db = this.Db.Omit(columns...)
	return this
}
func (this *MysqlDatabaseHandler) Having(query string, values ...interface{}) DatabaseHandler {
	this.Db = this.Db.Having(query, values...)
	return this
}
func (this *MysqlDatabaseHandler) Joins(query string, args ...interface{}) DatabaseHandler {
	this.Db = this.Db.Joins(query, args...)
	return this
}
func (this *MysqlDatabaseHandler) Find(out interface{}, where ...interface{}) DatabaseHandler {
	this.Db = this.Db.Find(out, where...)
	return this
}
func (this *MysqlDatabaseHandler) First(out interface{}, where ...interface{}) DatabaseHandler {
	this.Db = this.Db.First(out, where...)
	return this
}
func (this *MysqlDatabaseHandler) Last(out interface{}, where ...interface{}) DatabaseHandler {
	this.Db = this.Db.Last(out, where...)
	return this
}
func (this *MysqlDatabaseHandler) Row() *sql.Row {
	return this.Db.Row()
}
func (this *MysqlDatabaseHandler) Rows() (*sql.Rows, error) {
	return this.Db.Rows()
}
func (this *MysqlDatabaseHandler) Pluck(column string, value interface{}) DatabaseHandler {
	this.Db = this.Db.Pluck(column, value)
	return this
}
func (this *MysqlDatabaseHandler) Count(value interface{}) DatabaseHandler {
	this.Db = this.Db.Count(value)
	return this
}

func (this *MysqlDatabaseHandler) Related(value interface{}, foreignKeys ...string) DatabaseHandler {
	this.Db = this.Db.Related(value, foreignKeys...)
	return this
}
func (this *MysqlDatabaseHandler) Scan(dest interface{}) DatabaseHandler {
	this.Db = this.Db.Scan(dest)
	return this
}

// Update
func (this *MysqlDatabaseHandler) Update(attrs ...interface{}) DatabaseHandler {
	this.Db = this.Db.Update(attrs...)
	return this
}
func (this *MysqlDatabaseHandler) Updates(values interface{}, ignoreProtectedAttrs ...bool) DatabaseHandler {
	this.Db = this.Db.Updates(values)
	return this
}
func (this *MysqlDatabaseHandler) UpdateColumn(attrs ...interface{}) DatabaseHandler {
	this.Db = this.Db.UpdateColumn(attrs...)
	return this
}
func (this *MysqlDatabaseHandler) UpdateColumns(values interface{}) DatabaseHandler {
	this.Db = this.Db.UpdateColumns(values)
	return this
}
func (this *MysqlDatabaseHandler) Save(value interface{}) DatabaseHandler {
	this.Db = this.Db.Save(value)
	return this
}

// Insert
func (this *MysqlDatabaseHandler) Create(value interface{}) DatabaseHandler {
	this.Db = this.Db.Create(value)
	return this
}

// Delete
func (this *MysqlDatabaseHandler) Delete(value interface{}, where ...interface{}) DatabaseHandler {
	this.Db = this.Db.Delete(value, where...)
	return this
}

//Unscoped
func (this *MysqlDatabaseHandler) Unscoped() DatabaseHandler {
	this.Db = this.Db.Unscoped()
	return this
}

//Scoped
func (this *MysqlDatabaseHandler) Scopes(funcs ...func(*gorm.DB) *gorm.DB) DatabaseHandler {
	this.Db = this.Db.Scopes(funcs ...)
	return this
}

// Raw Sql
func (this *MysqlDatabaseHandler) Raw(sql string, values ...interface{}) DatabaseHandler {
	this.Db = this.Db.Raw(sql, values...)
	return this
}
func (this *MysqlDatabaseHandler) Exec(sql string, values ...interface{}) DatabaseHandler {
	this.Db = this.Db.Exec(sql, values...)
	return this
}

// Transaction
func (this *MysqlDatabaseHandler) Begin() DatabaseHandler {
	tx := &MysqlDatabaseHandler{
		Db: this.Db.Begin(),
	}
	return tx
}

func (this *MysqlDatabaseHandler) Commit() DatabaseHandler {
	this.Db = this.Db.Commit()
	return this
}

func (this *MysqlDatabaseHandler) Rollback() DatabaseHandler {
	this.Db = this.Db.Rollback()
	return this
}
