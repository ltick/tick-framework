filesystem 
===

## filesystem 模块

提供KV存储和LRU-KV存储（目前准备用在小文件合并大文件上）

### 参数说明

| 参数 | 定义 | 默认值 |
| --- | --- | --- |
| FILESYSTEM_PROVIDER | 存储类型 | "file" 可选"lruFile" |
| FILESYSTEM_DEFRAG_CONTENT_INTERVAL | 整理content文件时间间隔 | 30 * time.Minute |
| FILESYSTEM_DEFRAG_CONTENT_LIFETIME | lifeTime时间内不对content文件做整理操作 | 24 * time.Hour |
| FILESYSTEM_LRU_CAPACITY | lru时，容量大小，一个key容量为1 | 32 * 1024 * 1024 |


## block模块

block 类似Bitcast存储引擎的实现

> Bitcast存储引擎：由一棵hash tree在内存中管理全量的key，根据key可以获取value在磁盘文件上面的postion，进一步获取value本身的值。

只支持追加操作（Append-only），即所有的写操作只追加而不修改老的数据，每个文件都有一定的大小限制，当文件增加到相应的大小，就会产生一个新的文件，老的文件只读不写。在任意时刻，只有一个文件是可写的。

### 特点

- 写入流程比较简单，顺序写一次磁盘文件，更新hash
- 内存中全量索引Key值

### 参数说明

| 参数 | 定义 | 默认值 |
| --- | --- | --- |
| FILESYSTEM_BLOCK_DIR | 文件存储路径 | "/tmp/block" |
| FILESYSTEM_BLOCK_CONTENT_SIZE | 每一个文件大小限制 | 64 * 1024 * 1024 |
| FILESYSTEM_BLOCK_INDEX_SAVE_INTERVAL | index信息整理存盘时间间隔(类似redis的RDB, 但AOF已默认开启) | 5 * time.Minute |

### 部分实现细节

- index信息整理存盘时
    > 主要考虑：不影响原有内存中的索引信息（加锁操作）。
    1. 首先会将AOF复制写一份到buf。
    2. 然后会从原有index文件读取已有AOF全量数据，在内存中通过hash覆盖，再将数据写入硬盘Temp文件。
    3. 最后把buf中的数据追加到Temp文件。
    4. 切换index写入句柄。
    
