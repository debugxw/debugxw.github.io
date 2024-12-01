---
title: AQS之ReentrantLock
excerpt: 依赖于先进先出等待队列实现的阻塞锁和相关同步器
date: 2019-06-11 19:24:02
tags: 并发
toc: true
image: /img/theme/004.png
---

AbstractQueuedSynchronizer 简称 AQS，其依赖于先进先出等待队列实现了阻塞锁和相关的同步器。对于大多数依赖于某个整形值 status 来表示状态的同步器，都可以通过继承 AQS 来实现。AQS 定义了两种资源共享方式：独占和共享。子类只须重写其中的某些方法，并明确定义 status 在对象获得锁或释放锁时意味着什么，而其余的工作，例如维护等待列队，阻塞与唤醒等 AQS 都已经帮我们实现好了。


AQS 以模板方法模式在内部定义了获取和释放同步状态的模板方法，并留下钩子函数供子类继承时进行扩展，由子类决定在获取和释放同步状态时的细节，从而实现满足自身功能特性的需求。除此之外，AQS 通过内部的同步队列管理获取同步状态失败的线程，向实现者屏蔽了线程阻塞和唤醒的细节。
自定义同步器时，需要重新定义以下方法：
+ boolean tryAcquire(int): 独占锁模式下，尝试获取锁，成功返回 true，失败则返回 false
+ boolean tryRelease(int): 独占锁模式下，尝试释放锁，成功返回 true，失败则返回 false
+ int tryAcquireShared(int): 共享锁模式下，尝试获取锁。返回一个整形，负数代表获取失败；零代表获取成功，但已经没有剩余资源；正数代表获取成功，且还有可用资源
+ boolean tryReleaseShared(int): 共享锁模式下，尝试释放锁。如果释放后允许后续节点获取锁成功，返回 true，否则返回 false
+ isHeldExclusively(): 判断该线程是否正在独占资源。只有用到了 Condition 才去实现它

下面以 ReentrantLock 为例，从源码中分析如何通过 AQS 实现一个同步器。

### 一、ReentrantLock 概述
JDK 中独占锁的实现除了用 synchronized 外，还可以使用 ReentrantLock。虽然在性能上两者没有太大的区别，但是 ReentrantLock 相对于 synchronized 而言提供了较为丰富的功能，使用起来更加灵活，也更加适合复杂的并发场景。两者的比较：
+ 都是可重入锁
+ synchronized 表现为原生语法 (JVM) 层面的互斥锁，ReentrantLock 则表现为 API 层面的互斥锁（需要 lock() 和 unlock() 方法配合 try/finally 语句块来完成）
+ 相比 synchronized，ReentrantLock 增加了一些高级功能，主要有以下三项：**等待可中断，可实现公平锁，锁可以绑定多个条件（可实现选择性通知）**

### 二、AQS 的内部结构
AQS 内部主要有如下几个成员变量：
```java
    /**
     * The synchronization state.
     */
    private volatile int state;
    
    /**
     * Head of the wait queue, lazily initialized.  Except for
     * initialization, it is modified only via method setHead.  Note:
     * If head exists, its waitStatus is guaranteed not to be
     * CANCELLED.
     */
    private transient volatile Node head;       //指向同步等待队列的头
    
    /**
     * Tail of the wait queue, lazily initialized.  Modified only via
     * method enq to add new wait node.
     */
    private transient volatile Node tail;       //指向同步等待队列的尾
```
其中的 state 是一个带 volatile 的变量，表示线程同步状态，它在不同的组件中有不同的含义。例如在 ReentrantLock 中，state 代表锁被重入的次数。head 和 tail 表示 AQS 内部用于维护被阻塞线程的队列的头结点和尾节点。其中的 Node 是 AQS 的一个内部类，主要有以下几个成员变量：
```java
    volatile int waitStatus;        // 等待状态
    
    volatile Node prev;     // 指向前一个节点
    
    volatile Node next;     // 指向后一个节点
    
    volatile Thread thread;     // 当前节点所代表的线程
```
thread 代表当前节点所表示的线程，因为同步队列中的节点内部封装了之前竞争锁失败的线程，故而节点内部必然有一个对应线程实例的引用。waitStatus 代表节点的等待状态，一共有五个取值：
```java
        /** waitStatus value to indicate thread has cancelled */
        static final int CANCELLED =  1;
        /** waitStatus value to indicate successor's thread needs unparking */
        static final int SIGNAL    = -1;
        /** waitStatus value to indicate thread is waiting on condition */
        static final int CONDITION = -2;
        /**
         * waitStatus value to indicate the next acquireShared should
         * unconditionally propagate
         */
        static final int PROPAGATE = -3;
```
waitStatus 初始状态为 0。CANCELLED 表示在同步队列中等待的线程因为超时或者被中断，取消继续等待。SIGNAL 表示当前节点在释放锁后需要唤醒后续节点。CONDITION 表示当前节点正在某个 condition 上等待。PROPAGATE 表示下一次的共享模式同步状态的获取将会无条件的传播。

AQS 中还有一个表示持有同步状态的线程标志：
```java
    /**
     * The current owner of exclusive mode synchronization.
     */
    private transient Thread exclusiveOwnerThread;
```
这个变量继承自 AQS 的父类 AbstractOwnableSynchronizer，用来标识在独占同步模式下，锁被哪一个线程持有。

### 三、ReentrantLock 的内部类
ReentrantLock 有三个静态内部类 Sync、FairSync 和 NonfairSync。Sync 继承自 AQS，FairSync 和 NonFairSync 又继承自 Sync。因为 ReentrantLock 是独占锁，所以 FairSync 和 NonFairSync 重写了 AQS 中的 tryAcquire() 方法，而释放锁的方法 tryRelease() 在 Sync 中被重写。ReentrantLock 正是通过 FairSync 和 NonfairSync 来实现公平锁和非公平锁的。

### 四、ReentrantLock 非公平模式下的加锁流程
#### 1、构造器
```java
    public ReentrantLock() {
        sync = new NonfairSync();
    }
    
    public ReentrantLock(boolean fair) {
        sync = fair ? new FairSync() : new NonfairSync();
    }
```
ReentrantLock 只有这两个构造器，默认情况下使用非公平模式，如果想要使用公平锁，只需在构造时传入参数 true 即可。

#### 2、lock() 方法
加锁从 ReentrantLock.lock() 开始，而 ReentrantLock.lock() 在内部会直接调用 NonFairSync.lock()：
```java
    final void lock() {
        // 以 CAS 方式将 state 由 0 更新为 1
        if (compareAndSetState(0, 1))
            // 如果成功，则将当前线程设置为持有锁的线程，然后直接返回
            setExclusiveOwnerThread(Thread.currentThread());
        else
            acquire(1);     // 获取锁失败后执行该方法
    }
```
首先尝试快速获取锁，以 CAS 的方式将 state 的值更新为 1，只有当 state 的原值为 0 时更新才能成功，因为 state 在 ReentrantLock 的语境下等同于锁被线程重入的次数，这意味着只有当前锁未被任何线程持有时该方法才会返回成功。若获取锁成功，则将当前线程标记为持有锁的线程，然后整个加锁流程就结束了。若获取锁失败，则执行 acquire() 方法：
```java
    public final void acquire(int arg) {
        if (!tryAcquire(arg) &&
            acquireQueued(addWaiter(Node.EXCLUSIVE), arg))
            selfInterrupt();
    }
```
acquire() 方法是 AQS 中定义的方法，它封装了加锁流程中的主要处理逻辑。首先使用 tryAcquire() 尝试获取锁，如果失败则将当前线程封装成节点并添加到等待队列尾部。然后调用 acquireQueued() 让线程在等待队列中获取锁，直到获取锁成功才返回。如果在整个等待过程中被中断过，返回 true，否则返回 false。如果线程在等待过程中被中断，它不会立即响应。在获取到锁后才调用 selfInterrupt()，将中断补上。

#### 3、tryAcquire() 方法
该方法用于尝试获取独占锁，如果成功返回 true，失败则返回 false。AQS.tryAcquire() 源码如下：
```java
    protected boolean tryAcquire(int arg) {
        throw new UnsupportedOperationException();
    }
```
tryAcquire() 是 AQS 中定义的钩子函数，该方法默认会抛出异常。它会强制同步组件在通过 AQS 来实现同步功能时必须重写该方法。既然是钩子函数，那为什么不把它们声明为 abstract 方法，而是直接抛出异常呢？这是因为实现不同的同步器只需要用到 AQS 中定义的部分钩子函数，如果使用 abstract，那么自定义的同步器就需要重写 AQS 中的全部钩子函数，这显然是不合理的。所以 AQS 将一些需要子类覆写的方法都设计成 protect 方法，将其默认实现为抛出 UnsupportedOperationException 异常。如果子类使用到这些方法，但是没有覆写，则会抛出异常；如果子类没有使用到这些方法，则不需要做任何操作。这样子类就可以只重写那些要用得到的方法。
ReentrantLock 在公平和非公平模式下对此有不同的实现，非公平模式下 NonfairSync.tryAcquire() 的实现如下：
```java
    protected final boolean tryAcquire(int acquires) {
        return nonfairTryAcquire(acquires);
    }
```
底层直接调用到了 ReentrantLock.Sync.nonfairTryAcquire()，从名字上我们就可以知道这是非公平模式下尝试获取锁的实现方式：
```java
    /**
         * Performs non-fair tryLock.  tryAcquire is implemented in
         * subclasses, but both need nonfair try for trylock method.
         */
        final boolean nonfairTryAcquire(int acquires) {
            final Thread current = Thread.currentThread();
            int c = getState();
            if (c == 0) {       // 如果 state 为 0，说明锁尚未被任何线程占有
                if (compareAndSetState(0, acquires)) {
                    setExclusiveOwnerThread(current);       // 将当前线程标记为持有锁的线程
                    return true;
                }
            }
            else if (current == getExclusiveOwnerThread()) {        // 当前线程正持有锁，且锁将要被重入
                int nextc = c + acquires;
                if (nextc < 0) // overflow
                    throw new Error("Maximum lock count exceeded");
                setState(nextc);        // 设置 state，代表被重入的次数
                return true;
            }
            return false;
        }
```
这是非公平模式下获取锁的通用方式，它囊括了线程在获取锁时的所有情况：
+ 如果 state 为 0，说明锁未被任何线程持有，则以 CAS([更多关于 CAS](https://debugxw.github.io/2019/04/26/Compare-and-Swap/)) 的方式将 state 设置为 1，并将当前线程标记为持有锁的线程，获取锁成功，返回 true。如果 state 不为 0，则说明已有线程占有锁，执行第二步操作。或者 CAS 操作失败，说明在判断 state == 0 之后，CAS 操作之前有其他的线程占有了锁，返回 false。
+ 如果当前线程为正持有锁的线程，则将锁的重入次数加 1，返回 true。这里不再需要加锁，因为该线程之前已经获得了锁，所以这个累加操作不用再进行同步
+ 如果锁已被其它线程占有，返回 false。

因为 ReentrantLock 用 state 来统计锁被线程重入的次数，所以当前线程尝试获取锁的操作是否成功可以简化理解为：state 的值是否成功累加 1，是则尝试获取锁成功，否则尝试获取锁失败。既然 tryAcquire() 方法囊括了线程在尝试获取锁的所有情况，那么为什么在刚刚进入 NonFairSync.lock() 方法时还要通过 compareAndSetState(0, 1) 去尝试获取锁呢？我们完全可以把 compareAndSetState(0, 1)去掉，对最后的结果不会有任何影响。这种在进行通用逻辑处理之前针对某些特殊情况提前进行处理的方式在后面还会看到，一个直观的想法就是它能提升性能，而代价是牺牲一定的代码简洁性。

#### 4、addWaiter() 方法
```java
    /**
     * Creates and enqueues node for current thread and given mode.
     *
     * @param mode Node.EXCLUSIVE for exclusive, Node.SHARED for shared
     * @return the new node
     */
    private Node addWaiter(Node mode) {
        Node node = new Node(Thread.currentThread(), mode);
        // Try the fast path of enq; backup to full enq on failure
        Node pred = tail;
        if (pred != null) {
            node.prev = pred;
            // 如果 tail 不为空且 CAS 操作成功，则提前处理以提升性能
            if (compareAndSetTail(pred, node)) {
                pred.next = node;
                return node;
            }
        }
        enq(node);      // 包含所有的入队逻辑
        return node;
    }
```
AQS.addWaiter() 方法会将当前线程封装成一个新的节点，并添加到等待队列末尾。if 语句中的入队逻辑在 enq() 方法中也有，之所以加上这部分重复代码和尝试获取锁时的重复代码一样，对某些特殊情况进行特殊处理，牺牲一定的代码可读性来换取性能的提升。enq() 源码如下：
```java
    private Node enq(final Node node) {
        for (;;) {
            Node t = tail;
            if (t == null) { // Must initialize
                // 如果队列为空，则构造新节点，以 CAS 的方式设置新节点为队首元素
                if (compareAndSetHead(new Node()))
                    tail = head;
            } else {
                node.prev = t;
                // 队列不为空，以 CAS 的方式将 node 添加至队列末尾
                if (compareAndSetTail(t, node)) {
                    t.next = node;
                    return t;
                }
            }
        }
    }
```
外层的 for 循环相当于一个 CAS 自旋操作，它可以保证所有获取锁失败的线程经过失败重试后最后都能加入同步队列。注意当前线程所在的结点不能直接插入空队列，**因为阻塞的线程是由前驱结点进行唤醒的**。故先要插入一个结点作为队列首元素，当锁释放时由它来唤醒后面被阻塞的线程，在逻辑上这个队列首元素也可以表示当前已经获取锁的线程。

#### 5、acquireQueued() 方法
```java
/**
     * Acquires in exclusive uninterruptible mode for thread already in
     * queue. Used by condition wait methods as well as acquire.
     *
     * @param node the node
     * @param arg the acquire argument
     * @return {@code true} if interrupted while waiting
     */
    final boolean acquireQueued(final Node node, int arg) {
        boolean failed = true;
        try {
            boolean interrupted = false;
            for (;;) {
                final Node p = node.predecessor();      // 获得 node 的前驱节点
                // 如果前驱节点为 head，且尝试获取锁成功
                if (p == head && tryAcquire(arg)) {
                    setHead(node);
                    p.next = null; // help GC
                    failed = false;
                    return interrupted;
                }
                if (shouldParkAfterFailedAcquire(p, node) &&        // 判断是否要阻塞当前线程
                    parkAndCheckInterrupt())        // 阻塞当前线程，并检查线程被阻塞期间是否被中断
                    interrupted = true;
            }
        } finally {
            if (failed)
                cancelAcquire(node);
        }
    }
```
该方法会使线程在队列中等待，直到获取锁后才返回，在整个等待过程中线程并不响应中断，直到获取锁后才执行 selfInterrupt() 方法，将中断补上。在第一个 if 语句中，当前线程首先会判断前驱结点是否是头结点，当前线程的前驱节点为头结点时，有以下两种情况：
+ head 节点的 thread 为空（由前面的 addWaiter 方法保证了如果队列为空，则会新建一个 thread 为空的 head 节点）
+ head 节点的 thread 不为空，这时 head.thread 可能持有锁，也有可能已经释放锁

在这两种情况下，当前线程都会尝试去获取锁，如果成功获取锁则会设置当前结点为头结点并返回。

第二个 if 语句中首先会调用 shouldParkAfterFailedAcquire() 方法判断是否应该阻塞线程，内容如下：
```java
    private static boolean shouldParkAfterFailedAcquire(Node pred, Node node) {
        int ws = pred.waitStatus;
        if (ws == Node.SIGNAL)
            /*
             * This node has already set status asking a release
             * to signal it, so it can safely park.
             */
            return true;
        if (ws > 0) {   // waitStatus 为 CANCALLED
            /*
             * Predecessor was cancelled. Skip over predecessors and
             * indicate retry.
             */
            do {
                node.prev = pred = pred.prev;
            } while (pred.waitStatus > 0);
            pred.next = node;
        } else {    // 初始状态（ReentrantLock 语境下）
            /*
             * waitStatus must be 0 or PROPAGATE.  Indicate that we
             * need a signal, but don't park yet.  Caller will need to
             * retry to make sure it cannot acquire before parking.
             */
            compareAndSetWaitStatus(pred, ws, Node.SIGNAL);
        }
        return false;
    }
```
大致逻辑如下：
+ pre.waitStatus 为 SIGNAL，表示前驱节点释放锁后会唤醒当前节点，当前线程可以安全的被阻塞，返回 true
+ pre.waitStatus 为 CANCELED，即前驱节点线程任务被取消，为了保证当前线程阻塞后能够被唤醒，则一直往队列头部回溯直到找到一个状态不为 CANCELLED 的结点，然后将当前节点 node 连接在这个结点的后面
+ pre.waitStatus 为初始状态，则用 CAS 将 pre.waitStatus 设置为 SIGNAL

这个方法的大致思路是要确保当前结点的前驱结点的状态为 SIGNAL，SIGNAL 意味着线程释放锁后会唤醒后面阻塞的线程。**毕竟，只有确保能够被唤醒，当前线程才能放心的阻塞。**但是注意，只有上面的第一种情况才会直接返回 true 并阻塞线程，后两种情况都会返回 false 并重新执行 acquireQueued() 中的 for 循环。这种延迟阻塞其实也是一种高并发场景下的优化，试想如果锁被占用的时间非常短，那么就有可能在重新执行循环的时候成功获取了锁，这样的话线程阻塞与唤醒的开销就省了下来。

最后再来看一下阻塞线程的方法 parkAndCheckInterrupt()：
```java
    private final boolean parkAndCheckInterrupt() {
        LockSupport.park(this);
        return Thread.interrupted();
    }
```
很简单直接调用了 LockSupport.park() 阻塞线程（[更多关于 LockSupport](https://debugxw.github.io/2019/05/30/LockSupport/)），当线程被唤醒后，如果在阻塞过程中被中断，则返回 true，否则返回 false。

注意：Thread.interrupted() 方法会检查线程的中断状态，**并将中断状态清除。**所以在 acquireQueued() 方法中会有一个 interrupted 标志位来保存线程的中断状态，然后返回给 acquire() 方法，如果线程被中断过，那么就会在 acquire() 方法中调用 selfInterrupt() 方法将中断状态再次设置为 true。selfInterrupt() 方法如下：
```java
    static void selfInterrupt() {
        Thread.currentThread().interrupt();
    }
```

### 五、ReentrantLock 非公平模式下的解锁流程
解锁从 ReentrantLock.unlock() 开始，而 ReentrantLock.unlock() 会调用 AQS 中的 release() 方法：
```java
    public final boolean release(int arg) {
        if (tryRelease(arg)) {      // 尝试释放锁
            Node h = head;
            if (h != null && h.waitStatus != 0)
                unparkSuccessor(h);     // 唤醒等待列队中的一个线程
            return true;
        }
        return false;
    }
```
先来看 tryRelease() 方法：
```java
    protected final boolean tryRelease(int releases) {
            int c = getState() - releases;      // 持有锁次数减一
            if (Thread.currentThread() != getExclusiveOwnerThread())
                throw new IllegalMonitorStateException();
            boolean free = false;
            if (c == 0) {
                free = true;
                setExclusiveOwnerThread(null);      // 清除锁的持有线程标记
            }
            setState(c);        // 更新 state 的值
            return free;
        }
```
tryRelease() 也是 AQS 中定义的一个钩子函数，ReentrantLock 中的 Sync 类对其进行了重写。逻辑很简单，就是将线程持有锁的次数 state 减一，若减少后线程完全释放锁，即 state 为 0，返回 true，否则返回 false。由于执行该方法的线程必然持有锁，故不需要任何同步操作。

release() 方法中，如果 tryRelease() 方法返回 true，则表明锁可以被其他线程使用，应该唤醒后续等待线程。但在这之前，还应判断两个条件：
+ h != null 是为了防止队列为空，即没有任何线程处于等待队列中，那么也就不需要进行唤醒的操作
+ h.waitStatus != 0 是为了防止队列中虽有线程，但该线程还未阻塞，由前面的分析知，线程在阻塞自己前必须设置前驱结点的状态为 SIGNAL，否则它不会阻塞自己。

接下来就是唤醒操作 unparkSuccessor()：
```java
    private void unparkSuccessor(Node node) {
        /*
         * If status is negative (i.e., possibly needing signal) try
         * to clear in anticipation of signalling.  It is OK if this
         * fails or if status is changed by waiting thread.
         */
        int ws = node.waitStatus;
        if (ws < 0)
            compareAndSetWaitStatus(node, ws, 0);

        /*
         * Thread to unpark is held in successor, which is normally
         * just the next node.  But if cancelled or apparently null,
         * traverse backwards from tail to find the actual
         * non-cancelled successor.
         */
        Node s = node.next;
        if (s == null || s.waitStatus > 0) {
            s = null;
            for (Node t = tail; t != null && t != node; t = t.prev)
                if (t.waitStatus <= 0)
                    s = t;
        }
        if (s != null)
            LockSupport.unpark(s.thread);
    }
```
正常情况下只要唤醒后继结点的线程就行了，但是后继结点可能已经取消等待，所以从队列尾部往前回溯，找到离头结点最近的正常结点，并唤醒其线程。

### 六、公平锁和非公平锁的不同

在公平模式下，ReentrantLock 对锁的获取有较为严格的限制：在同步队列有线程等待的情况下，所有线程在获取所之前必须先加入同步队列。队列中的线程按加入队列的次序先后获取锁。
公平锁的加锁入口 FairSync.lock() 如下：
```java
    final void lock() {
        acquire(1);
    }
```
这里直接调用了 AQS.acquire() 方法，与非公平锁 NonFairSync.lock() 相比少了尝试获取锁的步骤，这是第一点不同。接着就是获取锁的通用方法 tryAcquire，该方法在线程未加入队列，加入队列阻塞前和阻塞后被唤醒时都会执行：
```java
    protected final boolean tryAcquire(int acquires) {
            final Thread current = Thread.currentThread();
            int c = getState();
            if (c == 0) {
                if (!hasQueuedPredecessors() &&
                    compareAndSetState(0, acquires)) {
                    setExclusiveOwnerThread(current);
                    return true;
                }
            }
            else if (current == getExclusiveOwnerThread()) {
                int nextc = c + acquires;
                if (nextc < 0)
                    throw new Error("Maximum lock count exceeded");
                setState(nextc);
                return true;
            }
            return false;
        }
```
可以看到，和非公平锁的 tryAcquire() 方法相比，该方法多了一个 hasQueuedPredecessors() 方法，源码如下：
```java
    public final boolean hasQueuedPredecessors() {
        // The correctness of this depends on head being initialized
        // before tail and on head.next being accurate if the current
        // thread is first in queue.
        Node t = tail; // Read fields in reverse initialization order
        Node h = head;
        Node s;
        return h != t &&
            ((s = h.next) == null || s.thread != Thread.currentThread());
    }
```
该方法会判断是否有优先级比自己高的线程在队列中等待获取锁。大致逻辑如下：
+ 如果 h == t，那么可以确定等待队列中一定没有优先级比自己高的线程在等待获取锁，直接返回 false
+ 如果 h != t 成立，那 (s = h.next) == null 就一定成立啊？为什么还要判断呢？**不要忘记，这时在多线程的环境下。**由之前的分析可以知道，一个节点入队分为三步：
  + 待添加节点 node 的 pre 指向 tail
  + CAS 操作更新 tail
  + 之前的 tail 的 next 指向 node
所以就有可能出现另一个线程 CAS 更新完 tail，还没来得及将之前的 tail 的 next 指向 node，时间片到了，这个线程开始执行 hasQueuedPredecessors() 方法，此时 (s = h.next) == null 就会为 true。
+ 如果上面所述的节点加入队列的三个步骤全部完成，即节点已成功加入队列，那么就需要使用 s.thread != Thread.currentThread() 来判断，这个节点是否是当前节点所在的线程。

简单理解：当前有优先级更高的线程在队列中等待，那么当前线程将不会执行 CAS 操作去获取锁，保证了线程获取锁的顺序与加入同步队列的顺序一致，很好的保证了公平性，但也增加了获取锁的成本。

#### 效率问题
AQS 内部使用 FIFO 实现同步等待队列，既然是先进先出，在公平锁模式下，一个线程尝试获取锁时就会判断同步等待队列中是否已有线程在等待获取锁，如果有则排到队尾，否则尝试获取锁，FIFO 的特性显然很符合公平锁。但同样是 FIFO，它如何实现非公平锁呢？从前面的非公平锁加锁流程中可以看到，线程在加入到同步等待队列之前有两次抢占锁的机会：
+ 第一次是非重入式的获取锁，只有在当前锁未被任何线程占有（包括自身）时才能成功（NonFairSync.lock() 中）
+ 第二次是在进入同步队列前，包含所有情况的获取锁的方式（AQS.acquire() 中调用 tryAcquire()）

只有这两次获取锁都失败后，线程才会构造结点并加入同步队列等待。而线程释放锁时是先释放锁（修改 state 值），然后才唤醒后继结点的线程的。试想下这种情况，线程 A 已经释放锁，但还没来得及唤醒后继线程 C，而这时另一个线程 B 刚好尝试获取锁，此时锁恰好不被任何线程持有，它将成功获取锁而不用加入队列等待。线程 C 被唤醒尝试获取锁，而此时锁已经被线程 B 抢占，故而其获取失败并继续在队列中等待。但在公平锁模式下，线程 B 就不得不加入到等待队列并阻塞，直到 C 唤醒 B。

非公平锁对锁的竞争是抢占式的（队列中线程除外），线程在进入等待队列前可以进行两次尝试，这大大增加了获取锁的机会。这种好处体现在两个方面：
+ 线程不必加入等待队列就可以获得锁，不仅免去了构造结点并加入队列的繁琐操作，同时也节省了线程阻塞唤醒的开销，线程阻塞和唤醒涉及到线程上下文的切换和操作系统的系统调用。是非常耗时的。在高并发情况下，如果线程持有锁的时间非常短，短到线程入队阻塞的过程超过线程持有并释放锁的时间开销，那么这种抢占式特性对并发性能的提升会更加明显。
+ 减少CAS竞争。如果线程必须要加入阻塞队列才能获取锁，那入队时 CAS 竞争将变得异常激烈，CAS 操作虽然不会导致失败线程挂起，但不断失败重试导致的对 CPU 的浪费也不能忽视。除此之外，加锁流程中至少有两处通过将某些特殊情况提前来减少 CAS 操作的竞争，增加并发情况下的性能。

### 七、等待可中断

等待可中断是指当持有锁的线程长期不释放锁的时候，正在等待的线程可以选择放弃等待，改为处理其他的事情，可中断特性对处理执行时间非常长的同步块非常有帮助。ReentrantLock 中的 lockInterruptibly() 方法可以实现该功能，lockInterruptibly 相当于是 lock() 的一个衍生品，其解锁方法也为 unlock()。lockInterruptibly 在内部直接调用 AQS 的 acquireInterruptibly() 方法:
```java
    public final void acquireInterruptibly(int arg)
            throws InterruptedException {
        // 如果在执行 tryAcquire() 之前线程已被中断，则直接抛出 InterruptedException
        if (Thread.interrupted())
            throw new InterruptedException();
        if (!tryAcquire(arg))
            doAcquireInterruptibly(arg);
    }
```
在执行 tryAcquire() 方法之前会先检查中断状态，判断线程是否已被中断。尝试获取锁的方法 tryAcquire() 在 lockInterruptibly() 和 lock() 中都是调用的同一个方法，并没有什么不同。不同的是 doAcquireInterruptibly() 方法：
```java
    private void doAcquireInterruptibly(int arg)
        throws InterruptedException {
        final Node node = addWaiter(Node.EXCLUSIVE);
        boolean failed = true;
        try {
            for (;;) {
                final Node p = node.predecessor();
                if (p == head && tryAcquire(arg)) {
                    setHead(node);
                    p.next = null; // help GC
                    failed = false;
                    return;
                }
                if (shouldParkAfterFailedAcquire(p, node) &&
                    parkAndCheckInterrupt())
                    throw new InterruptedException();
            }
        } finally {
            if (failed)
                cancelAcquire(node);
        }
    }
```
可以看到，该方法和 lock() 中调用的 acquireQueued() 方法在逻辑上大致相同，其中不同的一点是：在 for 循环中的第二个 if 语句中，如果判断条件为 true，那么直接抛出异常，然后由上一级调用者来处理。由前面的分析可知，如果 if 判断条件返回 true，那么就说明在线程阻塞过程中被中断过，则直接抛出异常，然后停止等待，直接返回。而在 acquireQueued() 方法中，如果线程在等待过程中有被中断过，它并不会抛出异常，而是设置一个中断状态标志，最后在成功获得锁之后将这个中断状态标志返回，由上一级调用来决定如何处理。这就是 doAcquireInterruptibly() 实现等待可中断的奥秘。

#### cancelAcquire() 方法
cancelAcquire() 方法会将代表该线程的节点从等待队列中删除：
```java
    private void cancelAcquire(Node node) {
        // Ignore if node doesn't exist
        if (node == null)
            return;

        node.thread = null;

        // Skip cancelled predecessors
        Node pred = node.prev;
        while (pred.waitStatus > 0)
            node.prev = pred = pred.prev;

        // predNext is the apparent node to unsplice. CASes below will
        // fail if not, in which case, we lost race vs another cancel
        // or signal, so no further action is necessary.
        Node predNext = pred.next;

        // Can use unconditional write instead of CAS here.
        // After this atomic step, other Nodes can skip past us.
        // Before, we are free of interference from other threads.
        node.waitStatus = Node.CANCELLED;

        // If we are the tail, remove ourselves.
        if (node == tail && compareAndSetTail(node, pred)) {
            compareAndSetNext(pred, predNext, null);
        } else {
            // If successor needs signal, try to set pred's next-link
            // so it will get one. Otherwise wake it up to propagate.
            int ws;
            if (pred != head &&
                ((ws = pred.waitStatus) == Node.SIGNAL ||
                 (ws <= 0 && compareAndSetWaitStatus(pred, ws, Node.SIGNAL))) &&
                pred.thread != null) {
                Node next = node.next;
                if (next != null && next.waitStatus <= 0)
                    compareAndSetNext(pred, predNext, next);
            } else {
                unparkSuccessor(node);
            }

            node.next = node; // help GC
        }
    }
```

> 参考：
  [从源码角度彻底理解ReentrantLock(重入锁)](https://www.cnblogs.com/takumicx/p/9402021.html)

