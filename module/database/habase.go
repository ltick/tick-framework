package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/tsuna/gohbase"
	"github.com/tsuna/gohbase/hrpc"
	"time"
)

var (
	errHbaseRegister          = "database(hbase): initiate error"
	errHbaseNewDatabase       = "database(hbase): new database error"
	errHbaseScan              = "database(hbase): scan error"
	errHbaseScanNext          = "database(hbase): scan next error"
	errHbaseScanClose         = "database(hbase): scan close error"
	errHbaseGet               = "database(hbase): get error"
	errHbaseRecordNotFound    = "database(hbase): record not found"
	errHbasePut               = "database(hbase): put error"
	errHbaseDelete            = "database(hbase): delete error"
	errHbaseAppend            = "database(hbase): append error"
	errHbaseIncrement         = "database(hbase): increment error"
	errHbaseCheckAndPut       = "database(hbase): check and put error"
	errHbaseGetConnection     = "database(hbase): get connection error"
	errHbaseDatabaseNotExists = "database(hbase): database not exists"
)

const (
	DEFAULT_MAX_ACTIVE = 30
	DEFAULT_TIMEOUT    = 120
)

type HbaseHandler struct {
	// connection
	databases map[string]*HbaseDatabaseHandler
}
type HbaseDatabaseHandler struct {
	Client    chan gohbase.Client
	MaxIdle   int
	MaxActive int
	Timeout   time.Duration
	Host      string
	User      string
	Password  string
	closed    bool
	active    int
}

type HbaseDatabaseCallback struct {
	Callback hrpc.Call
}

type HbaseDatabaseScanner struct {
	Scanner hrpc.Scanner
}

func NewHbaseHandler() NosqlHandler {
	return &HbaseHandler{}
}

// 批量生成连接，并把连接放到连接池channel里面
func (this *HbaseHandler) Initiate(ctx context.Context) error {
	this.databases = make(map[string]*HbaseDatabaseHandler)
	return nil
}

func (this *HbaseHandler) NewDatabase(ctx context.Context, name string, config map[string]interface{}) (NosqlDatabaseHandler, error) {
	db := &HbaseDatabaseHandler{}
	db.MaxActive = DEFAULT_MAX_ACTIVE
	db.Timeout = DEFAULT_TIMEOUT
	configHost := config["DATABASE_HBASE_HOST"]
	if configHost != nil {
		host, ok := configHost.(string)
		if ok {
			db.Host = host
		} else {
			return nil, errors.New(errHbaseNewDatabase + ": invalid config DATABASE_HBASE_HOST")
		}
		if host == "" {
			return nil, errors.New(errHbaseNewDatabase + ": empty DATABASE_HBASE_HOST")
		}
	} else {
		return nil, errors.New(errHbaseNewDatabase + ": empty DATABASE_HBASE_HOST")
	}
	configTimeout, ok := config["DATABASE_HBASE_TIMEOUT"]
	if ok && configTimeout != nil {
		timeoutString, ok := configTimeout.(string)
		if ok && timeoutString != "" {
			timeout, err := time.ParseDuration(timeoutString)
			if err != nil {
				return nil, errors.New(errHbaseNewDatabase + ": invalid config DATABASE_HBASE_TIMEOUT :" + err.Error())
			}
			db.Timeout = timeout
		} else {
			return nil, errors.New(errHbaseNewDatabase + ": invalid config DATABASE_HBASE_TIMEOUT")
		}
	}
	configMaxActive, ok := config["DATABASE_HBASE_MAX_ACTIVE"]
	if ok && configMaxActive != nil {
		maxIdleConns, ok := configMaxActive.(int)
		if ok {
			if maxIdleConns != 0 {
				db.MaxActive = maxIdleConns
			}
		} else {
			return nil, errors.New(errHbaseNewDatabase + ": invalid config DATABASE_HBASE_MAX_ACTIVE")
		}
	}
	db.Client = make(chan gohbase.Client, db.MaxActive)
	for x := 0; x < db.MaxActive; x++ {
		client := gohbase.NewClient(db.Host)
		if client == nil {
			return nil, errors.New(errHbaseNewDatabase + ": new client error")
		}
		db.Client <- client
	}
	if this.databases == nil {
		this.databases = make(map[string]*HbaseDatabaseHandler)
	}
	this.databases[name] = db
	return db, nil
}

// 从连接池里取出连接
func (this *HbaseHandler) GetDatabase(name string) (NosqlDatabaseHandler, error) {
	handlerDatabase, ok := this.databases[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf(errHbaseDatabaseNotExists, name))
	}
	return handlerDatabase, nil
}

func (this *HbaseDatabaseScanner) Next() (*hrpc.Result, error) {
	result, err := this.Scanner.Next()
	if err != nil {
		return nil, errors.New(errHbaseScanNext + ": " + err.Error())
	}
	return result, nil
}
func (this *HbaseDatabaseScanner) Close() error {
	err := this.Scanner.Close()
	if err != nil {
		return errors.New(errHbaseScanClose + ": " + err.Error())
	}
	return nil
}

func (this *HbaseDatabaseHandler) GetConnection() (client interface{}, err error) {
	if len(this.Client) == 0 {
		newClient := gohbase.NewClient(this.Host)
		if newClient == nil {
			return nil, errors.New(errHbaseGetConnection + ": new client error")
		}
		this.Client <- newClient
	}
	client = <-this.Client
	return client, nil
}

// 回收连接到连接池
func (this *HbaseDatabaseHandler) ReleaseConnection(client interface{}) {
	hbaseClient, ok := client.(gohbase.Client)
	if ok {
		if len(this.Client) == this.MaxActive {
			hbaseClient.Close()
			return
		}
		this.Client <- hbaseClient
	}
}

func (this *HbaseDatabaseHandler) GetConnectionPoolSize() int {
	return len(this.Client)
}

func (this *HbaseDatabaseHandler) Scan(ctx context.Context, table string) (NosqlDatabaseScanner, error) {
	client, err := this.GetConnection()
	if err != nil {
		return nil, errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return nil, errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(hbaseClient)
	scanRequest, err := hrpc.NewScanStr(ctx, table)
	if err != nil {
		return nil, errors.New(errHbaseScan + ": " + err.Error())
	}
	scanner := &HbaseDatabaseScanner{hbaseClient.Scan(scanRequest)}
	return scanner, nil
}
func (this *HbaseDatabaseHandler) Get(ctx context.Context, table string, key string) ([]map[string]string, error) {
	client, err := this.GetConnection()
	if err != nil {
		return nil, errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return nil, errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(hbaseClient)
	getRequest, err := hrpc.NewGetStr(ctx, table, key)
	if err != nil {
		return nil, errors.New(errHbaseGet + ": " + err.Error())
	}
	result, err := hbaseClient.Get(getRequest)
	if err != nil || *result.Exists {
		return nil, errors.New(errHbaseRecordNotFound + ": " + err.Error())
	}
	cells := make([]map[string]string, len(result.Cells))
	for index, column := range result.Cells {
		cell := make(map[string]string, 0)
		cell["row"] = string(column.Row)
		cell["family"] = string(column.Family)
		cell["qualifier"] = string(column.Qualifier)
		cell["value"] = string(column.Value)
		cells[index] = cell
	}
	return cells, nil
}
func (this *HbaseDatabaseHandler) Put(ctx context.Context, table string, key string, values map[string]map[string][]byte) (err error) {
	client, err := this.GetConnection()
	if err != nil {
		return errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(hbaseClient)
	// Values maps a ColumnFamily -> Qualifiers -> Values.
	putRequest, err := hrpc.NewPutStr(ctx, table, key, values)
	if err != nil {
		return errors.New(errHbasePut + ": " + err.Error())
	}
	_, err = hbaseClient.Put(putRequest)
	if err != nil {
		return errors.New(errHbasePut + ": " + err.Error())
	}
	return nil
}
func (this *HbaseDatabaseHandler) Delete(ctx context.Context, table string, key string, values map[string]map[string][]byte) (err error) {
	client, err := this.GetConnection()
	if err != nil {
		return errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(client)
	deleteRequest, err := hrpc.NewDelStr(ctx, table, key, values)
	if err != nil {
		return errors.New(errHbaseDelete + ": " + err.Error())
	}
	_, err = hbaseClient.Delete(deleteRequest)
	if err != nil {
		return errors.New(errHbaseDelete + ": " + err.Error())
	}
	return nil
}
func (this *HbaseDatabaseHandler) Append(ctx context.Context, table string, key string, values map[string]map[string][]byte) error {
	client, err := this.GetConnection()
	if err != nil {
		return errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(client)
	appendRequest, err := hrpc.NewAppStr(ctx, table, key, values)
	if err != nil {
		return errors.New(errHbaseAppend + ": " + err.Error())
	}
	_, err = hbaseClient.Append(appendRequest)
	if err != nil {
		return errors.New(errHbaseAppend + ": " + err.Error())
	}
	return nil
}
func (this *HbaseDatabaseHandler) Increment(ctx context.Context, table string, key string, values map[string]map[string][]byte) (int64, error) {
	client, err := this.GetConnection()
	if err != nil {
		return 0, errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return 0, errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(client)
	incrementRequest, err := hrpc.NewIncStr(ctx, table, key, values)
	if err != nil {
		return 0, errors.New(errHbaseIncrement + ": " + err.Error())
	}
	i, err := hbaseClient.Increment(incrementRequest)
	if err != nil {
		return 0, errors.New(errHbaseIncrement + ": " + err.Error())
	}
	return i, nil
}
func (this *HbaseDatabaseHandler) CheckAndPut(ctx context.Context, table string, key string, values map[string]map[string][]byte, family string, qualifier string, expectedValue []byte) (bool, error) {
	client, err := this.GetConnection()
	if err != nil {
		return false, errors.New(errHbaseScan + ": " + err.Error())
	}
	hbaseClient, ok := client.(gohbase.Client)
	if !ok {
		return false, errors.New(errHbaseScan + ": invalid client type")
	}
	// 使用完把连接回收到连接池里
	defer this.ReleaseConnection(client)
	putRequest, err := hrpc.NewPutStr(ctx, table, key, values)
	if err != nil {
		return false, errors.New(errHbaseCheckAndPut + ": " + err.Error())
	}
	b, err := hbaseClient.CheckAndPut(putRequest, family, qualifier, expectedValue)
	if err != nil {
		return false, errors.New(errHbaseCheckAndPut + ": " + err.Error())
	}
	return b, nil
}
func (this *HbaseDatabaseHandler) Close() {
	client, _ := this.GetConnection()
	for client != nil {
		hbaseClient, ok := client.(gohbase.Client)
		if !ok {
			return
		}
		hbaseClient.Close()
		// 使用完把连接回收到连接池里
		this.ReleaseConnection(client)
		client, _ = this.GetConnection()
	}
	return
}
