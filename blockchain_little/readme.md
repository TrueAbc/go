
- 了解blockchain的整体结构
- golang进行简单的实现


1. block结构

    | 字段 | 解释 |
    | --- | --- |
    | Timestamp | 时间辍 |
    | PrevBlockHash | 前一块hash |
    | Hash | 当前块hash |
    | Data | 交易数据 |

    - SetHash 现在是计算PreBlockHash, timestamp, data 的sha256

2. Blockchain 结构
   - 现有仅仅实现为一个Block的数组
   - 加上一个添加区块的函数

二者之间都要有New函数以及创世块函数
