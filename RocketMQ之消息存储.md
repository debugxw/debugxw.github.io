---
title: RocketMQ之消息存储
excerpt: 基于CommitLog的存储机制，支持多种故障转移机制，保证高效、可靠的消息传输
date: 2023-05-14
tags: 消息队列
toc: true
image: /img/theme/001.png
---

## 一、消息存储整体架构

RocketMQ作为现代消息队列中的佼佼者，其本质上还是一个分布式的存储系统，而决定一个存储系统性能的好坏，最主要的就是它的存储结构。先来看一下RocketMQ消息存储的整体架构：

<center><img src="../../../../img/rocket_mq/store_design.png" width="100%" height="100%" /></center>

消息存储架构图中主要有下面三个跟消息存储相关的文件

### 1.1、CommitLog

CommitLog文件存储了Broker收到的所有消息，消息以顺序写入的方式追加到该文件末尾，如果文件写满了，就写入下一个文件。单个文件大小默认1G，文件名长度为20位，左边补零，剩余为文件起始偏移量，例如第一个文件名为00000000000000000000，第二个文件名为00000000001073741824（1G=1073741824）。为什么要将文件名设置为起始偏移量呢？这就涉及到了后文要介绍的ConsumeQueue

### 1.2、ConsumeQueue

ConsumeQueue文件其本质上是一个索引文件，引入的目的主要是提高消息消费和查询的性能。既然是索引文件，那么自然而然ConsumeQueue中存放的就是消息在CommitLog中的偏移量，这也就解释了为什么CommitLog文件的文件名要设置为起始偏移量了

ConsumeQueue中每个索引条目的大小都是固定的，由三个部分组成：
+ 消息在CommitLog中的偏移位置，占用8个字节
+ 消息的大小，占用4个字节
+ 消息tag的哈希值（用于过滤消息），占用8个字节

单个ConsumeQueue文件由30W个条目组成，可以像数组一样随机访问每一个条目，每个ConsumeQueue文件大小约为5.72M。其存储路径为`$HOME/store/consumequeue/{topic}/{queueId}/{fileName}`

### 1.3、IndexFile

IndexFile本质上也是一个索引文件，它提供了一种可以通过key或时间区间来查询消息的方法。其存储路径为`$HOME/store/index/{fileName}`，文件名fileName是以创建时的时间戳命名的，固定的单个IndexFile文件大小约为400M，一个IndexFile可以保存2000W个索引，IndexFile的底层存储设计为在文件系统中实现HashMap结构，故RocketMQ的索引文件其底层实现为hash索引

以上就是RocketMQ的整体存储结构，其主要以文件的形式来存储数据，并通过索引文件来提升消息消费和查询的效率。但是文件始终是保存在磁盘上面的，我们知道，磁盘的I/O是特别慢的，大多数系统的性能瓶颈也都是在磁盘I/O上面。那为什么RocketMQ将数据存储在磁盘上面，还能实现单机10W+的吞吐量呢？这就涉及到了**页缓存**和**内存映射**

## 二、页缓存和内存映射

### 2.1、页缓存

页缓存（PageCache）是OS对文件的缓存，用于加速对文件的读写。操作系统会将一部分的内存用作PageCache。对于数据的写入，OS会先写入至PageCache内，随后通过异步的方式由pdflush内核线程将PageCache内的数据刷盘至物理磁盘上。对于数据的读取，如果读取文件时出现未命中PageCache的情况，OS将会从物理磁盘上将文件读取至PageCache，同时还会顺序对其它相邻块的数据文件进行预读取

在RocketMQ中，ConsumeQueue逻辑消费队列存储的数据较少，并且是顺序读取，在PageCache机制的预读取作用下，ConsumeQueue文件的读性能几乎接近读内存，即使在有消息堆积情况下也不会影响性能

### 2.2、内存映射

简单来说，内存映射就是将磁盘文件直接映射到用户应用程序内存空间，这样就**减少了传统I/O中数据在操作系统内核地址空间的缓冲区和用户应用程序地址空间的缓冲区之间来回进行拷贝的性能开销**

RocketMQ主要通过MappedByteBuffer对文件进行读写操作。其中，利用了[NIO中的FileChannel](https://www.cnblogs.com/robothy/p/14235598.html)模型将磁盘上的物理文件直接映射到用户态的内存地址中，将对文件的操作转化为直接对内存地址进行操作，从而极大地提高了文件的读写效率

**正因为需要使用内存映射机制，故RocketMQ的文件存储都使用定长结构来存储，方便一次将整个文件映射至内存**

## 三、Q&A

### 3.1、ConsumeQueue异步写入失败怎么办？

在读源码的过程中发现，ConsumeQueue是通过异步的方式构建的，也就是说只要主流程写CommitLog成功，就会给Producer返回写入消息成功。那如果程序异常宕机，ConsumeQueue异步构建失败，这条消息还怎么被消费者消费呢？

```java
// org.apache.rocketmq.store.DefaultMessageStore.ReputMessageService#run
public void run() {
    ...
    while (!this.isStopped()) {
        try {
            Thread.sleep(1);
            this.doReput();
        } catch (Exception e) {
            DefaultMessageStore.log.warn(this.getServiceName() + " service has exception. ", e);
        }
    }
    ...
}

// org.apache.rocketmq.store.DefaultMessageStore.ReputMessageService#doReput
private void doReput() {
    ...
    // 获取CommitLog中存储的新消息
    DispatchRequest dispatchRequest =
            DefaultMessageStore.this.commitLog.checkMessageAndReturnSize(result.getByteBuffer(), false, false);
    int size = dispatchRequest.getBufferSize() == -1 ? dispatchRequest.getMsgSize() : dispatchRequest.getBufferSize();

    if (dispatchRequest.isSuccess()) {
        if (size > 0) {
            // 如果有新消息，则分别调用 CommitLogDispatcherBuildConsumeQueue、CommitLogDispatcherBuildIndex
            // 进行构建 ConsumeQueue 和 IndexFile
            DefaultMessageStore.this.doDispatch(dispatchRequest);
            ...
        }
        ...
    }
    ...
}
```

原来，在Broker启动的时候，会判断上次进程结束是否异常，然后加载CommitLog、ConsumeQueue等数据文件，如果文件加载成功，就调用recover方法，做数据修正，保证数据的一致性

```java
// org.apache.rocketmq.store.DefaultMessageStore#load
public boolean load() {
    boolean result = true;

    try {
        // 判断上次程序退出是否异常
        boolean lastExitOK = !this.isTempFileExist();
        
        ...

        // load Commit Log
        result = result && this.commitLog.load();
        // load Consume Queue
        result = result && this.loadConsumeQueue();
        if (result) {
            this.storeCheckpoint =
                new StoreCheckpoint(StorePathConfigHelper.getStoreCheckpoint(this.messageStoreConfig.getStorePathRootDir()));
            this.indexService.load(lastExitOK);
            // 数据恢复
            this.recover(lastExitOK);
            log.info("load over, and the max phy offset = {}", this.getMaxPhyOffset());
        }
    } catch (Exception e) {
        log.error("load exception", e);
        result = false;
    }

    ...

    return result;
}

// org.apache.rocketmq.store.DefaultMessageStore#recover
private void recover(final boolean lastExitOK) {
    long maxPhyOffsetOfConsumeQueue = this.recoverConsumeQueue();

    if (lastExitOK) {
        this.commitLog.recoverNormally(maxPhyOffsetOfConsumeQueue);
    } else {
        this.commitLog.recoverAbnormally(maxPhyOffsetOfConsumeQueue);
    }

    this.recoverTopicQueueTable();
}
```

> 参考：  
    [RocketMQ官方文档](https://github.com/apache/rocketmq/blob/develop/docs/cn/design.md)  
    [Java NIO文件通道FileChannel](https://www.cnblogs.com/robothy/p/14235598.html)  
    [RocketMQ源码分析-深入消息存储（1）](http://antzuhl.cn/archives/rocketmqstore1)  
    [rocketmq之ConsumeQueue学习笔记](https://my.oschina.net/u/5480812/blog/8761857)