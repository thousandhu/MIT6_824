GFS 阅读笔记



## 设计预期

设计预期往往针对系统的应用场景，是系统在不同选择间做balance的重要依据，对于理解GFS在系统设计时为何做出现有的决策至关重要。所以我们应重点关注：

-   失效是常态
-   主要针对大文件
-   读操作：大规模流式读取、小规模随机读取
-   写操作：大规模顺序追加写，写入后很少修改
-   高效明确定义的并行追加写
-   稳定高效地网络带宽

## 整体设计

### 1、系统架构

GFS主要由以下三个系统模块组成：

-   Master：管理元数据、整体协调系统活动
-   ChunkServer：存储维护数据块（Chunk），读写文件数据
-   Client：向Master请求元数据，并根据元数据访问对应ChunkServer的Chunk

![](http://img.blog.csdn.net/20140114135050781?watermark/2/text/aHR0cDovL2Jsb2cuY3Nkbi5uZXQvdGFvdGFvdGhlcmlwcGVy/font/5a6L5L2T/fontsize/400/fill/I0JBQkFCMA==/dissolve/70/gravity/SouthEast)

### 2、设计要点

#### 1）Chunk尺寸

由于GFS主要面向大文件存储和大规模读写操作，所以其选择了远大于一般文件系统的64MB的Chunk尺寸。

好处：

-   减少元数据量，便于客户端预读缓存，减少客户端访问Master的次数，减小Master负载；
-   减少元数据量，Master可以将元数据放在内存中；
-   客户端短时间内工作集落在同一Chunk上的概率更高，减少客户端访问不同ChunkServer建立TCP连接的次数，从而减少网络负载。

坏处：

-   易产生数据碎片；
-   小文件占用Chunk少，对小文件的频繁访问会集中在少数ChunkServer上，从而产生小文件访问热点。

可能的解决方案：

-   增大小文件复制参数；
-   客户端间互传数据。

#### 2）元数据存储方式

Master存储的元数据包括：命名空间、文件和Chunk的对应关系、Chunk位置信息。

命名空间、文件和Chunk的对应关系的存储方式：

-   内存：真实数据；
-   磁盘：定期Checkpoint（压缩B树）和上次CheckPoint后的操作日志；
-   多机备份：Checkpoint文件和操作日志。

Chunk位置信息的存储方式：

-   内存：真实数据
-   磁盘：不持久存储

系统启动和新Chunk服务器加入时从Chunk服务器获取。避免了Master与ChunkServer之间的数据同步，只有Chunk服务器才能最终确定Chunk是否在它的硬盘上。

#### 3）Log

log是非常关键的数据，log会存在多个不同的主机上，并且只有当刷新了这个相关的操作记录到本地和远程磁盘后，才会给客户端操作应答。Master的日志在超过某一大小后，执行checkpoint操作，卸载自己的状态。

#### 4）一致性模型

命名空间修改：原子性和正确性

文件数据修改（之后详细解释）：

![image](http://img.blog.csdn.net/20140114135105406?watermark/2/text/aHR0cDovL2Jsb2cuY3Nkbi5uZXQvdGFvdGFvdGhlcmlwcGVy/font/5a6L5L2T/fontsize/400/fill/I0JBQkFCMA==/dissolve/70/gravity/SouthEast)

-   修改失败：不一致
-   串行随机写：一致且已定义
-   并行随机写：非原子写，一致但未定义
-   串行/并行追加写（推荐）：少量不一致，保证已定义

## 详细设计

### 1、系统交互设计

#### 1）重要原则

最小化所有操作与Master的交互。

#### 2）缓存机制

最小化读取操作与Master的交互：

客户端访问Chunk前从Master获取元数据的过程中，会预取和缓存部分Chunk的元数据，从而减少与Master的交互。

#### 2）租约机制

最小化变更操作与Master的交互：

Master收到变更操作请求后

-   选择一个Chunk副本发放定时租约作为主Chunk并返回给客户端；
-   客户端与主Chunk进行通信进行变更操作；
-   租约超时前，对该Chunk的变更操作都由主Chunk进行序列化控制。

#### 3）数据流与控制流分离

![image](http://img.blog.csdn.net/20140114135126062?watermark/2/text/aHR0cDovL2Jsb2cuY3Nkbi5uZXQvdGFvdGFvdGhlcmlwcGVy/font/5a6L5L2T/fontsize/400/fill/I0JBQkFCMA==/dissolve/70/gravity/SouthEast)

数据推送与控制操作同时进行。

数据流：管道方式按最优化的Chunk服务器链推送，以最大化带宽，最小化延时（全双工交换网络，每台机器进出带宽最大化）；数据会被缓存在chunk的cache中直到使用或者过期。

控制流：

-   master选择出primary chunk （1-4）
-   主ChunkServer确定所有副chunk接收完毕后，对所有变更操作分配连续序列号（序号确定操作顺序）；
-   按序修改本地数据；
-   请求二级副本按序修改；
-   所有副本修改完成成功，否则失败重做。

#### 4）原子记录追加

追加写时GFS推荐的写入方式，应重点研究。

a. 原子记录追加：客户端指定写入数据，GFS返回真实写入偏移量。

b. 追加写的过程：

-   追加操作会使Chunk超过尺寸填充当前Chunk；通知二级副本做同样操作；通知客户机向新Chunk追加；
-   追加操作不会使Chunk超过尺寸主Chunk追加数据；通知二级副本写在相同位置上；成功 - 返回偏移； 失败 - 再次操作。

c. 追加结果：失败的追加操作可能导致Chunk间字节级别不一致，但当最终追加成功后，所有副本在返回的偏移位置一致已定义，之后的追加操作不受影响。如下图所示：

![image](http://img.blog.csdn.net/20140114135137109?watermark/2/text/aHR0cDovL2Jsb2cuY3Nkbi5uZXQvdGFvdGFvdGhlcmlwcGVy/font/5a6L5L2T/fontsize/400/fill/I0JBQkFCMA==/dissolve/70/gravity/SouthEast)

d. 冗余数据处理：对于追加写产生的冗余数据

-   Chunk尺寸不足时的填充数据
-   追加失败时产生的重复内容

可在插入数据时附带记录级别的Checksum或唯一标识符，在客户端读取数据时进行校验过滤。

#### 6）快照

使用COW技术，瞬间完成。快照实现的过程：

-   收回文件所有Chunk的租约；
-   操作元数据完成元数据拷贝；
-   客户端要写入该文件的Chunk时，Master通知该Chunk所在ChunkServer进行本地拷贝；
-   发放租约给拷贝Chunk；
-   返回拷贝Chunk的位置信息。

### 2、Master节点设计
这里没有讲如何防止出现两个master，这应该是后面课程会单独讲的。

#### 1）名称空间管理

多操作并行，名称空间锁保证执行顺序，文件操作需获得父目录读锁和目标文件/目录写锁。（在名字上的读锁保证父目录不被删除）

#### 2）副本位置

Chunk跨机架分布：

好处 -

-   防止整个机架破坏造成数据失效
-   综合利用多机架整合带宽（机架出入带宽可能小于机架上机器的总带宽，所以应最大化每台机架的带宽利用率）；

坏处 - 写操作需跨机架通信。

#### 3）Chunk管理

a. 创建操作，主要考虑：

-   平衡硬盘使用率；
-   限制单ChunkServer短期创建次数（创建开销虽小，但每次创建往往意味着大量的后续写入）；
-   跨机架分布。

b. 重复制，即有效副本不足时，通过复制增加副本数。优先考虑：

-   副本数量和复制因数相差多的；
-   live（未被删除）文件的；
-   阻塞客户机处理的

Chunk进行重复制。策略与创建类似。

c. 重负载均衡，通过调整副本位置，平衡格机负载。策略与创建类似。新ChunkServer将被逐渐填满。

#### 4）垃圾回收

惰性回收空间：删除操作仅在文件名后添加隐藏标记，Master在常规扫描中删除超时隐藏文件的元数据，并通知对应ChunkServer删除Chunk。

好处 -

-   与创建失败的无效Chunk一致的处理方式；
-   批量执行开销分散，Master相对空闲时进行；
-   删除操作一定时间内可逆转。

坏处 - 不便于用户进行存储空间调优。

解决方案 - 再次删除加速回收，不同命名空间不同复制回收策略。

#### 5）过期失效副本检测

过期检测：Master维护Chunk级别版本号（在master为chunk发放一个令牌是，会增加chunk的版本号并通知最新的副本），新租约增加Chunk版本号，并通知所有副本更新版本号，过期Chunk会因版本号过旧被检测。

### 3、容错机制设计

#### 1）高可用性

主要的提高可用性的机制：

-   组件快速恢复
-   Chunk复制
-   Master服务器复制Checkpoint和操作日志多机备份；Master进程失效重启，硬件失效则新机器重启Master进程；“影子”Master，通过操作日志“模仿”主Master操作，元数据版本稍慢。作用 - 增强对于不是很活跃修改的文件的读取能力，提高了对于读取脏数据也ok的应用的读取性能。

#### 2）数据完整性

ChunkServer独立维护CheckSum检验副本完整性。原因：

-   跨Chunk服务器比较副本开销大；
-   追加操作造成的的byte级别不一致，导致无法通过比较副本判断完整性。

Chunk**读取**和Chunk服务器**空闲**时，进行CheckSum校验，发现损坏Chunk上报Master，进行重复制。

Checksum校验的开销：

-   Checksum读取开销；
-   Checksum校验计算开销。

但整体开销可以接受，特别是对追加写操作。

覆盖写与追加写的Checksum计算开销比较。两者的关键区别在于不完整块的CheckSum计算：

-   追加写 - 对最后一个不完整块，在写入后进行增量的CheckSum计算。New CheckSum由Old CheckSum和新数据算出，未对原有数据重新计算，原有数据损坏，依然可以在后续读取时发现；
-   覆盖写 - 对第一个和最后一个不完整块，在写之前进行CheckSum校验。否则，覆盖写只能重新对整块计算CheckSum，若写操作未覆盖部分存在损坏数据，新CheckSum将从包含损坏数据的Chunk算出，之后再也无法校验出该损坏数据。



# 参考文献

基于这篇http://blog.csdn.net/taotaotheripper/article/details/18261395笔记进行了修改，原笔记写的不错。

