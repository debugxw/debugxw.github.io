---
title: AQS之ConditionObject
excerpt: Java中Lock实现的一部分，用于等待/通知模型中的条件变量管理
date: 2019-07-10 14:53:07
tags: [Java,并发]
image: /img/theme/013.jpg
---

我们知道任何一个java对象，都拥有一组监视器方法，主要包括wait()、notify()、notifyAll()方法，这些方法与synchronized关键字配合使用可以实现等待/通知机制。同样，java中的Condition与Lock配合使用也可以实现类似的等待/通知机制。

### 一、使用示例
```java
class BoundedBuffer {
    final Lock lock = new ReentrantLock();
    final Condition notFull = lock.newCondition();
    final Condition notEmpty = lock.newCondition();

    final Object[] items = new Object[100];
    int putptr, takeptr, count;

    public void put(Object x) throws InterruptedException {
        lock.lock();
        try {
            while (count == items.length)
                notFull.await();
            items[putptr] = x;
            if (++putptr == items.length) putptr = 0;
            ++count;
            notEmpty.signal();
        } finally {
            lock.unlock();
        }
    }

    public Object take() throws InterruptedException {
        lock.lock();
        try {
            while (count == 0)
                notEmpty.await();
            Object x = items[takeptr];
            if (++takeptr == items.length) takeptr = 0;
            --count;
            notFull.signal();
            return x;
        } finally {
            lock.unlock();
        }
    }
}
```
这是 java 官方文档提供的例子，是一个典型的生产者-消费者模型。这里在同一个 lock 锁上，创建了两个条件队列 notFull，notEmpty。当数组已满，没有存储空间时，put 方法在 notFull 条件上等待，直到数组 notFull 条件成立。当数组空了，没有数据可读时，take 方法在 notEmpty 条件上等待，直到数组 notEmpty 条件成立，而 notEmpty.signal() 和 notFull.signal() 则用来唤醒等待在这两个条件上的线程。

### 二、同步列队(sync queue)和条件列队(condition queue)
AQS 内部的同步列队 ([更多参考](https://debugxw.github.io/2019/06/11/AQS%E4%B9%8BReentrantLock/)) 用于存储所有竞争锁失败的线程，其实现为一个双向链表。而每一个 Condition 对象都会对应一个条件列队，每个条件列队都是一个单向列表。同步列队和条件列队中的节点都是 AbstractQueuedSynchronizer.Node 类型，在同步列队中我们使用 prev 和 next 来串联节点，而在条件列队中使用 nextWaiter 来串联节点。AbstractQueuedSynchronizer.Node 的具体成员如下：
```java
    volatile Node prev;     // 指向 sync queue 的前一个节点
    volatile Node next;     // 指向 sync queue 的下一个节点
    
    Node nextWaiter;        // 指向 condition queue 的下一个节点
    
    volatile Thread thread;     // 当前节点所代表的线程
    
    volatile int waitStatus;    // 节点的状态
    // 节点的状态的初始化为 0，只有以下几个取值
    static final int CANCELLED =  1;
    static final int SIGNAL    = -1;
    static final int CONDITION = -2;
    static final int PROPAGATE = -3;
```

#### sync queue和condition queue的联系
一般情况下，等待锁的 sync queue 和条件队列 condition queue 是相互独立的，彼此之间并没有任何关系。但是，当我们调用某个条件队列的 signal 方法时，会将某个或所有等待在这个条件队列中的线程唤醒，被唤醒的线程和普通线程一样需要去争锁，如果没有抢到，则同样要被加到等待锁的 sync queue 中去，此时节点就从 condition queue 中被转移到 sync queue 中。这里尤其要注意的是，node 是被一个一个转移过去的，哪怕我们调用的是 signalAll() 方法也是一个一个转移过去的，而不是将整个条件队列接在 sync queue 的末尾。

我们在 sync queue 中只使用 prev、next 来串联链表，而不使用 nextWaiter；我们在 condition queue 中只使用 nextWaiter 来串联链表，而不使用 prev、next。实际上它们是两个完全独立的链表，只不过是使用了相同的数据结构 AQS.Node 作为链表的节点而已。因此，将节点从 condition queue 中转移到 sync queue 中时，我们需要断开原来的链接 (nextWaiter) 并建立新的链接 (prev, next)，这在某种程度上也是需要将节点一个一个地转移过去的原因之一。

### 三、ConditionObject 的核心属性
```java
    /** First node of condition queue. */
    private transient Node firstWaiter;
    /** Last node of condition queue. */
    private transient Node lastWaiter;
```
这两个节点分别代表 condition queue 的队头和队尾。

### 四、ConditionObject 方法解析

#### 1、await()
```java
public final void await() throws InterruptedException {
    // 如果当前线程在调用 await() 方法前已经被中断了，则直接抛出 InterruptedException
    if (Thread.interrupted())
        throw new InterruptedException();
    Node node = addConditionWaiter();   // 将当前线程封装成 Node 添加到条件队列
    int savedState = fullyRelease(node);    // 释放当前线程所占用的锁，保存当前的锁状态
    int interruptMode = 0;
    // 如果当前线程不在同步队列中，说明刚刚被 await, 还没有人调用 signal 方法，则直接将当前线程挂起
    while (!isOnSyncQueue(node)) {
        LockSupport.park(this); // 线程将在这里被挂起，停止运行
        // 能执行到这里说明要么是 signal 方法被调用了，要么是线程被中断了
        // 所以检查下线程被唤醒的原因，如果是因为中断被唤醒，则跳出 while 循环
        if ((interruptMode = checkInterruptWhileWaiting(node)) != 0)
            break;
    }
    if (acquireQueued(node, savedState) && interruptMode != THROW_IE)
        interruptMode = REINTERRUPT;
    if (node.nextWaiter != null) // clean up if cancelled
        unlinkCancelledWaiters();
    if (interruptMode != 0)
        reportInterruptAfterWait(interruptMode);
}
```
await() 方法会响应中断，所以在刚进入方法时会检查当前线程是否已被中断，如果是则直接抛出异常。然后调用 addConditionWaiter() 将当前线程封装成 Node 节点并加入到 condition queue。

#### 2、addConditionWaiter()
```java
private Node addConditionWaiter() {
    Node t = lastWaiter;
    // If lastWaiter is cancelled, clean out.
    if (t != null && t.waitStatus != Node.CONDITION) {
        unlinkCancelledWaiters();
        t = lastWaiter;
    }
    Node node = new Node(Thread.currentThread(), Node.CONDITION);
    if (t == null)
        firstWaiter = node;
    else
        t.nextWaiter = node;
    lastWaiter = node;
    return node;
}
```
首先我们考虑一下存在两个线程同时入队吗？肯定不存在。因为既然能调用 await() 方法，那么该线程必然已经获得了锁，而在独占锁模式下获得锁的线程同一时间只有一个，所以这里并不存在并发，没有必要使用 CAS 操作。

在这个方法中，只是简单的将当前线程封装成 Node 加到条件队列的末尾。这和将一个线程封装成 Node 加入 sync queue 略有不同：
1. 节点加入 sync queue 时 waitStatus 的值为 0，但节点加入 condition queue 时 waitStatus 的值为 Node.CONDTION
2. sync queue 中的头节点只起唤醒后续节点的功能，如果队列为空，则会先创建一个 thread 为空的头节点，再创建一个代表当前节点的 Node 添加在头节点的后面；而 condtion queue 在初始化时，直接将 firstWaiter 和 lastWaiter 指向新建的节点就行了
3. sync queue 是一个双向队列，在节点入队后，要同时修改当前节点的前驱和前驱节点的后继；而在 condtion queue 中，我们只修改了前驱节点的 nextWaiter，也就是说，condtion queue 是作为单向队列来使用的

如果入队时发现尾节点已经取消等待了，那么我们就不应该接在它的后面，此时需要调用 unlinkCancelledWaiters 来剔除那些已经取消等待的线程：
```java
private void unlinkCancelledWaiters() {
    Node t = firstWaiter;
    Node trail = null;
    while (t != null) {
        Node next = t.nextWaiter;
        if (t.waitStatus != Node.CONDITION) {
            t.nextWaiter = null;
            if (trail == null)
                firstWaiter = next;
            else
                trail.nextWaiter = next;
            if (next == null)
                lastWaiter = trail;
        }
        else
            trail = t;
        t = next;
    }
}
```
该方法将从头节点开始遍历整个队列，剔除其中 waitStatus 不为 Node.CONDTION 的节点。

#### 3、fullyRelease()
在节点被成功添加到条件队列的末尾后，将调用 fullyRelease() 来释放当前线程所占用的锁：
```java
final int fullyRelease(Node node) {
    boolean failed = true;
    try {
        int savedState = getState();    // 保存线程锁的状态
        if (release(savedState)) {      // 释放锁
            failed = false;
            return savedState;
        } else {
            throw new IllegalMonitorStateException();
        }
    } finally {
        if (failed)
            node.waitStatus = Node.CANCELLED;
    }
}
```
首先，当执行这个方法时，说明当前线程已经被封装成 Node 扔进条件队列了。在该方法中，通过调用 release() 方法释放锁 ([更多关于 release 参考](https://debugxw.github.io/2019/06/11/AQS%E4%B9%8BReentrantLock/))。值得注意的是，这是一次性释放了所有的锁，即对于可重入锁而言，无论重入了几次，这里是一次性释放完的，这也就是为什么该方法的名字叫 fullyRelease。但这里尤其要注意的是 release(savedState) 方法是有可能抛出 IllegalMonitorStateException 的。这是因为当前线程可能并不是持有锁的线程，而只有持有锁的线程才能调用 await 方法。既然 fullyRelease 方法在 await 方法中，为啥当前线程还有可能并不是持有锁的线程呢？这是因为在调用 await 方法时，其实并没有检测 Thread.currentThread() == getExclusiveOwnerThread()，换句话说，也就是执行到 fullyRelease 这一步，才会去检测这一点，而这一点检测是由 AQS 子类实现 tryRelease 方法来保证的。例如，ReentrantLock 对 tryRelease 方法的实现如下：
```java
protected final boolean tryRelease(int releases) {
    int c = getState() - releases;
    // 检查当前线程是否持有锁
    if (Thread.currentThread() != getExclusiveOwnerThread())
        throw new IllegalMonitorStateException();
    boolean free = false;
    if (c == 0) {
        free = true;
        setExclusiveOwnerThread(null);
    }
    setState(c);
    return free;
}
```
在 fullyRelease() 中，当发现当前线程不是持有锁的线程时，就会进入 finally 块，将当前 Node 的状态设为 Node.CANCELLED，这也就是为什么上面的 addConditionWaiter 在添加新节点前每次都会检查尾节点是否已经被取消的一方面原因。

在当前线程的锁被完全释放了之后，就可以调用 LockSupport.park(this) 把当前线程挂起，等待被 signal。但是，在挂起当前线程之前需要先用 isOnSyncQueue 确保它不在 sync queue 中，这是为什么呢？当前线程不是在一个和 sync queue 无关的条件队列中吗？怎么可能会出现在 sync queue 中呢？为了解释这一问题，我们先来看一下 singnalAll 方法。

#### 4、signalAll()
在看 signalAll() 之前，首先要区分调用 signalAll 方法的线程与 signalAll 方法要唤醒的线程：
+ 调用 signalAll 方法的线程本身是已经持有了锁，现在准备释放锁了
+ 在条件队列里的线程是已经在对应的条件上挂起了，等待着被 signal 唤醒，然后去争锁

```java
public final void signalAll() {
    if (!isHeldExclusively())   // 检查当前线程是否是持有锁的线程
        throw new IllegalMonitorStateException();
    Node first = firstWaiter;
    if (first != null)      // 如果条件列队不为空，则调用 doSignalAll() 唤醒所有节点
        doSignalAll(first);
}
```
首先，该方法会调用 isHeldExclusively() 检查当前线程是否是持有锁的线程，该方法由继承 AQS 的子类实现，例如在 ReentrantLock 中的实现如下：
```java
    protected final boolean isHeldExclusively() {
        // While we must in general read state before owner,
        // we don't need to do so to check if current thread is owner
        return getExclusiveOwnerThread() == Thread.currentThread();
    }
```

#### 5、doSignalAll()
```java
private void doSignalAll(Node first) {
    lastWaiter = firstWaiter = null;
    do {
        Node next = first.nextWaiter;
        first.nextWaiter = null;
        transferForSignal(first);
        first = next;
    } while (first != null);
}
```
首先我们通过 lastWaiter = firstWaiter = null 将整个条件队列清空，然后通过一个 do-while 循环，将原先的条件队列里面的节点一个一个拿出来，再通过 transferForSignal 方法一个一个添加到 sync queue 的末尾：
```java
final boolean transferForSignal(Node node) {
    // 如果该节点在调用 signal 方法前已经被取消了，则直接跳过这个节点
    if (!compareAndSetWaitStatus(node, Node.CONDITION, 0))
        return false;

    // 如果该节点在条件队列中正常等待，则利用 enq 方法将该节点添加至 sync queue 的尾部
    Node p = enq(node);
    int ws = p.waitStatus;
    if (ws > 0 || !compareAndSetWaitStatus(p, ws, Node.SIGNAL))
        LockSupport.unpark(node.thread);
    return true;
}
```
在 transferForSignal 方法中，我们先使用 CAS 操作将当前节点的 waitStatus 状态由 CONDTION 设为 0，如果修改不成功，则说明该节点已经被取消了，则直接返回，操作下一个节点；如果修改成功，则说明已经将该节点从等待的条件队列中成功 "唤醒" 了，但此时该节点对应的线程并没有真正被唤醒，它还要和其他普通线程一样去争锁，因此它将被添加到 sync queue 的末尾等待获取锁。

这里通过 enq 方法将该节点添加进 sync queue 的末尾。不过这里尤其注意的是，enq 方法将 node 节点添加进队列时，返回的是 node 的前驱节点。在 sync queue 中的节点都要靠前驱节点去唤醒，所以，这里要做的就是将前驱节点的 waitStatus 设为 Node.SIGNAL。这一点和 shouldParkAfterFailedAcquire() 所做的工作类似：
```java
private static boolean shouldParkAfterFailedAcquire(Node pred, Node node) {
    int ws = pred.waitStatus;
    if (ws == Node.SIGNAL)
        return true;
    if (ws > 0) {
        do {
            node.prev = pred = pred.prev;
        } while (pred.waitStatus > 0);
        pred.next = node;
    } else {
        compareAndSetWaitStatus(pred, ws, Node.SIGNAL);
    }
    return false;
}
```
所不同的是，shouldParkAfterFailedAcquire 将会向前查找，跳过那些被 cancel 的节点，然后将找到的第一个没有被 cancel 的节点的 waitStatus 设成 SIGNAL，最后再挂起。而在 transferForSignal 中，当前 Node 所代表的线程本身就已经被挂起了，所以这里做的更像是一个复合操作——**只要前驱节点处于被取消的状态或者无法将前驱节点的状态修成 Node.SIGNAL，那就将 Node 所代表的线程唤醒**，但这个条件并不意味着当前 lock 处于可获取的状态，有可能线程被唤醒了，但是锁还是被占有的状态，不过这样做至少是无害的，因为我们在线程被唤醒后还要去争锁，如果抢不到锁，则大不了再次被挂起。
值得注意的是，transferForSignal 是有返回值的，但是在 doSignalAll 方法中并没有用到，它将在 signal 方法中被使用。

signalAll() 方法流程：
1. 将条件队列清空，只是令 lastWaiter = firstWaiter = null，队列中的节点和连接关系仍然还存在
2. 取出条件列队中的每一个节点，使之成为孤立节点（nextWaiter，prev，next 属性都设置为 null）
3. 如果节点处于 CANCELLED 状态，则直接跳过该节点，由于是孤立节点，则会被 GC 回收
4. 如果节点处于正常状态，则通过 enq 方法将它添加到 sync queue 的末尾
5. 判断是否需要将该节点唤醒（包括设置该节点的前驱节点的状态为 SIGNAL），如有必要，直接唤醒该节点

#### 6、signal()
```java
public final void signal() {
    if (!isHeldExclusively())
        throw new IllegalMonitorStateException();
    Node first = firstWaiter;
    if (first != null)
        doSignal(first);
}
```
signal() 大致流程和 signalAll() 类似，只不过 signal() 只会唤醒一个节点，对于 AQS 的实现来说，就是唤醒条件队列中第一个没有被 Cancel 的节点。首先 signal() 依然是检查当前线程是不是已经持有了锁，这一点和上面的 signalAll() 方法一样，所不一样的是，接下来调用的是 doSignal 方法：
```java
private void doSignal(Node first) {
    do {
        if ((firstWaiter = first.nextWaiter) == null)
            lastWaiter = null;
        first.nextWaiter = null;
    } while (!transferForSignal(first) &&
                (first = firstWaiter) != null);
}
```
这个方法也是一个 do-while 循环，目的是遍历整个条件队列，找到第一个没有被 cancelled 的节点，并将它添加到条件队列的末尾。如果条件队列里面已经没有节点了，则将条件队列清空 (firstWaiter = lasterWaiter = null)。
在这里将节点由 condition queue 转移到 sync queue 依旧用的是 transferForSignal()，但是用到了它的返回值，只要节点被成功添加到 sync queue 中，transferForSignal 就返回 true，此时 while 循环的条件就不满足了，整个方法就此结束。

当线程等待的某一个条件满足时，它会被另一个线程唤醒，唤醒之后从哪里开始执行呢？当然是在哪里被挂起，就在哪里被唤醒并继续执行。以 await() 为例：
```java
    public final void await() throws InterruptedException {
        if (Thread.interrupted())
            throw new InterruptedException();
        Node node = addConditionWaiter();
        int savedState = fullyRelease(node);
        int interruptMode = 0;
        while (!isOnSyncQueue(node)) {
            LockSupport.park(this);     // 在这里被挂起了，被唤醒后，将从这里继续往下运行
            if ((interruptMode = checkInterruptWhileWaiting(node)) != 0)
                break;
        }
        if (acquireQueued(node, savedState) && interruptMode != THROW_IE)
            interruptMode = REINTERRUPT;
        if (node.nextWaiter != null) // clean up if cancelled
            unlinkCancelledWaiters();
        if (interruptMode != 0)
            reportInterruptAfterWait(interruptMode);
    }
```
这里值得注意的是，当线程被唤醒时，其实并不知道是因为什么原因被唤醒的，有可能是因为其他线程调用了 signal 方法，也有可能是因为当前线程被中断了。但是，无论是被中断唤醒还是被 signal 唤醒，被唤醒的线程最后都将离开 condition queue，进入到 sync queue 中。
随后，线程将在 sync queue 中利用 acquireQueued 方法进行 "阻塞式" 争锁，抢到锁就返回，抢不到锁就继续被挂起。因此，当 await() 方法返回时，必然是保证了当前线程已经持有了 lock 锁。

#### 7、中断的处理
如果线程在调用 await() 的过程中被中断，那么该如何处理呢？
在 java 中，中断对于当前线程只是个建议，由当前线程决定怎么对其做出处理。acquireQueued 方法对中断是不响应的，只是简单的记录抢锁过程中的中断状态，并在抢到锁后将这个中断状态返回，交于上层调用的函数处理，而这里 "上层调用的函数" 就是 await() 方法。那么 await() 方法是怎么对待这个中断的呢？这取决于：**中断发生时，线程是否已经被 signal 过？**

如果中断发生时，当前线程并没有被 signal 过，则说明当前线程还处于条件队列中，属于正常在等待中的状态，此时中断将导致当前线程的正常等待行为被打断，进入到 sync queue 中抢锁，因此，从 await 方法返回后，需要抛出 InterruptedException，表示当前线程因为中断而被唤醒。

如果中断发生时，当前线程已经被 signal 过了，则说明这个中断来的太晚了，既然当前线程已经被 signal 过了，那么就说明在中断发生前，它就已经正常地从 condition queue 中被唤醒了，所以随后即使发生了中断（注意，这个中断可以发生在抢锁之前，也可以发生在抢锁的过程中），我们都将忽略它，仅仅是在 await() 方法返回后，再自我中断，补一下这个中断。就好像这个中断是在 await() 方法调用结束之后才发生的一样。这里之所以要补一下这个中断，是因为在用 Thread.interrupted() 方法检测是否发生中断的同时，线程的中断状态会被清除，因此如果选择了忽略中断，则应该在 await() 方法退出后将它设成原来的样子。

理清了概念之后，我们再来看看 await() 方法是怎么做的，它用中断模式 interruptMode 这个变量记录中断事件，该变量有三个值：
+ **0：代表整个过程中一直没有中断发生**
+ **THROW_IE：表示退出 await() 方法时需要抛出 InterruptedException，这种模式对应于中断发生在 signal 之前**
+ **REINTERRUPT：表示退出 await() 方法时只需要再自我中断以下，这种模式对应于中断发生在 signal 之后，即中断来的太晚了**

```java
/** Mode meaning to reinterrupt on exit from wait */
private static final int REINTERRUPT =  1;
/** Mode meaning to throw InterruptedException on exit from wait */
private static final int THROW_IE    = -1;
```

#### 8、情况1：中断发生时，线程还没有被 signal 过
线程被唤醒后，我们将首先使用 checkInterruptWhileWaiting 方法检测中断的模式。
```java
private int checkInterruptWhileWaiting(Node node) {
    return Thread.interrupted() ?
        (transferAfterCancelledWait(node) ? THROW_IE : REINTERRUPT) : 0;
}
```
这里假设已经发生过中断，则 Thread.interrupted() 方法必然返回 true，接下来就是用 transferAfterCancelledWait 进一步判断是否发生了 signal：
```java
final boolean transferAfterCancelledWait(Node node) {
    if (compareAndSetWaitStatus(node, Node.CONDITION, 0)) {
        enq(node);
        return true;
    }
    /*
     * If we lost out to a signal(), then we can't proceed
     * until it finishes its enq().  Cancelling during an
     * incomplete transfer is both rare and transient, so just
     * spin.
     */
    while (!isOnSyncQueue(node))
        Thread.yield();
    return false;
}
```
判断一个 node 是否被 signal 过，一个简单有效的方法就是判断它是否离开了 condition queue，进入到 sync queue 中。换句话说，只要一个节点的 waitStatus 还是 Node.CONDITION，那就说明它还没有被 signal 过。假设中断发生时，线程还没有被 signal 过，则当前节点的 waitStatus 必然是 Node.CONDITION，则会成功执行 compareAndSetWaitStatus(node, Node.CONDITION, 0)，将该节点的状态设置成 0，然后调用 enq(node) 方法将当前节点添加进 sync queue 中，然后返回 true。
这里值得注意的是，此时并没有断开 node 的 nextWaiter，所以最后一定不要忘记将这个链接断开。

再回到 transferAfterCancelledWait 的调用处，可知，由于 transferAfterCancelledWait 将返回 true，现在 checkInterruptWhileWaiting 将返回 THROW_IE，这表示我们在离开 await 方法时应当要抛出 THROW_IE 异常。
再回到 checkInterruptWhileWaiting 的调用处：
```java
public final void await() throws InterruptedException {
    if (Thread.interrupted())
        throw new InterruptedException();
    Node node = addConditionWaiter();
    int savedState = fullyRelease(node);
    int interruptMode = 0;
    while (!isOnSyncQueue(node)) {
        LockSupport.park(this);
        if ((interruptMode = checkInterruptWhileWaiting(node)) != 0)    // 现在在这里！！！
            break;
    }
    if (acquireQueued(node, savedState) && interruptMode != THROW_IE)
        interruptMode = REINTERRUPT;
    if (node.nextWaiter != null) // clean up if cancelled
        unlinkCancelledWaiters();
    if (interruptMode != 0)
        reportInterruptAfterWait(interruptMode);
}
```
interruptMode 现在为 THROW_IE，则将执行 break，跳出 while 循环。接下将将执行 acquireQueued(node, savedState) 进行争锁，注意，这里传入的需要获取锁的重入数量是 savedState，即之前释放了多少，这里就需要再次获取多少。（[更多关于 acquireQueue() 方法参考](https://debugxw.github.io/2019/06/11/AQS%E4%B9%8BReentrantLock/)）

acquireQueued 是一个阻塞式的方法，获取到锁则退出，获取不到锁则会被挂起。该方法只有在最终获取到了锁后，才会退出，并且退出时会返回当前线程的中断状态，如果在获取锁的过程中又被中断了，则会返回 true，否则会返回 false。但是其实这里返回 true 还是 false 已经不重要了，因为前面已经发生过中断了，我们就是因为中断而被唤醒的不是吗？所以无论如何，我们在退出 await() 方法时，必然会抛出 InterruptedException。

这里假设它获取到了锁，则它将回到上面的调用处，由于这时的 interruptMode = THROW_IE，则会跳过if语句。接下来将执行：
```java
    if (node.nextWaiter != null) // clean up if cancelled
        unlinkCancelledWaiters();
```
前面说过，当前节点的 nextWaiter 是有值的，它并没有和原来的 condition 队列断开，当前线程已经获取到了锁，并从 sync queue 移除了，接下来应当将它从 condition 队列里面移除。由于 condition 队列是一个单向队列，我们无法获取到它的前驱节点，所以只能从头开始遍历整个条件队列，然后找到这个节点，再移除它。

然而，事实上呢，我们并没有这么做。因为既然已经必须从头开始遍历链表了，就干脆一次性把链表中所有没有在等待的节点都拿出去，所以这里调用了 unlinkCancelledWaiters 方法，该方法在前面已经讲过了，它就是简单的遍历链表，找到所有 waitStatus 不为 CONDITION 的节点，并把它们从队列中移除。

节点被移除后，接下来就是最后一步了——汇报中断状态：
```java
    if (interruptMode != 0)
        reportInterruptAfterWait(interruptMode);
```
```java
private void reportInterruptAfterWait(int interruptMode)
    throws InterruptedException {
    if (interruptMode == THROW_IE)
        throw new InterruptedException();
    else if (interruptMode == REINTERRUPT)
        selfInterrupt();
}
```
由此可以看出，一个调用了 await 方法被挂起的线程在被中断后不会立即抛出 InterruptedException，而是会被添加到 sync queue 中去争锁，如果争不到，还是会被挂起。只有争到了锁之后，该线程才得以从 sync queue 和 condition queue 中移除，最后抛出 InterruptedException。

所以说，**一个调用了 await 方法的线程，即使被中断了，它依旧还是会被阻塞住，直到它获取到锁之后才能返回，并在返回时抛出 InterruptedException。中断对它意义更多的是体现在将它从 condition queue 中移除，加入到 sync queue 中去争锁，从这个层面上看，中断和 signal 的效果其实很像，所不同的是，在 await() 方法返回后，如果是因为中断被唤醒，则 await() 方法需要抛出 InterruptedException 异常，表示是它是被非正常唤醒的（正常唤醒是指被 signal 唤醒）**。

#### 9、情况2：中断发生时，线程已经被 signal 过了
这种情况对应于中断来的太晚了，即 REINTERRUPT 模式，我们在拿到锁退出 await() 方法后，只需要再自我中断一下，不需要抛出 InterruptedException。值得注意的是，这种情况其实包含了两种子情况：
+ 中断发生在 signal 之后，加入到 acquireQueued 抢占锁之前
+ 中断发生在 acquireQueued 抢占锁的过程中

**情况2.1：中断发生在 signal 之后，加入到 acquireQueued 抢占锁之前**

对于这种情况，与前面中断发生于 signal 之前的主要差别在于 transferAfterCancelledWait 方法：
```java
final boolean transferAfterCancelledWait(Node node) {
    if (compareAndSetWaitStatus(node, Node.CONDITION, 0)) { // 线程 A 执行到这里，CAS 操作将会失败
        enq(node);
        return true;
    }
    
    // 由于中断发生前，线程已经被 signal 了，则这里只需要等待线程成功进入 sync queue 即可
    while (!isOnSyncQueue(node))
        Thread.yield();
    return false;
}
```
在这里，由于  signal 已经发生过了，则由之前分析的 signal 方法可知，此时当前节点的 waitStatus 必定不为 Node.CONDITION，它将跳过 if 语句。此时当前线程可能已经在 sync queue 中，或者正在进入到 sync queue 的路上。

为什么这里会出现 "正在进入到 sync queue 的路上" 的情况呢？
假设当前线程为线程 A，它被唤醒之后检测到发生了中断，来到了 transferAfterCancelledWait 这里，而另一个线程 B 在这之前已经调用了 signal 方法，该方法会调用 transferForSignal 将当前线程添加到 sync queue 的末尾。
```java
final boolean transferForSignal(Node node) {
    if (!compareAndSetWaitStatus(node, Node.CONDITION, 0))  // 线程 B 执行到这里，CAS 操作将会成功
        return false;

    Node p = enq(node);
    int ws = p.waitStatus;
    if (ws > 0 || !compareAndSetWaitStatus(p, ws, Node.SIGNAL))
        LockSupport.unpark(node.thread);
    return true;
}
```
因为线程 A 和线程 B 是并发执行的，而这里我们分析的是 "中断发生在 signal 之后"，则此时，线程 B 的 compareAndSetWaitStatus 先于线程 A 执行。这时可能出现线程 B 已经成功修改了 node 的 waitStatus 状态，但是还没来得及调用 enq(node) 方法，线程 A 就执行到了 transferAfterCancelledWait 方法，此时它发现 waitStatus 已经不是 Condition，但是其实当前节点还没有被添加到 sync node 队列中，因此，它接下来将通过自旋，等待线程 B 执行完 transferForSignal 方法。

#### 10、isOnSyncQueue()
线程 A 在自旋过程中会不断地判断节点有没有被成功添加进 sync queue，判断的方法就是 isOnSyncQueue：
```java
final boolean isOnSyncQueue(Node node) {
    if (node.waitStatus == Node.CONDITION || node.prev == null)
        return false;
    if (node.next != null) // If has successor, it must be on queue
        return true;
    return findNodeFromTail(node);
}
```
该方法很好理解，只要 waitStatus 的值还为 Node.CONDITION，则它一定还在 condtion 队列中，自然不可能在 sync queue 里面，而每一个调用了 enq 方法入队的线程：
```java
private Node enq(final Node node) {
    for (;;) {
        Node t = tail;
        if (t == null) { // Must initialize
            if (compareAndSetHead(new Node()))
                tail = head;
        } else {
            node.prev = t;
            if (compareAndSetTail(t, node)) {   // 即使这一步失败了 next.prev 一定是有值的
                t.next = node;  // 如果 t.next 有值，说明上面的 compareAndSetTail 方法一定成功了，则当前节点成为了新的尾节点
                return t;   // 返回了当前节点的前驱节点
            }
        }
    }
}
```
哪怕在设置 compareAndSetTail 这一步失败了，它的 prev 必然也是有值的，因此这两个条件只要有一个满足，就说明节点必然不在 sync queue 队列中。
另一方面，如果 node.next 有值，则说明它不仅在 sync queue 中，并且在它后面还有别的节点，则只要它有值，该节点必然在 sync queue 中。
如果以上都不满足，说明这里出现了尾部分叉（[尾部分叉参见这里](https://segmentfault.com/a/1190000015739343#articleHeader12))的情况，那么就调用 findNodeFromTail() 从尾节点向前寻找这个节点：
```java
private boolean findNodeFromTail(Node node) {
    Node t = tail;
    for (;;) {
        if (t == node)
            return true;
        if (t == null)
            return false;
        t = t.prev;
    }
}
```
这里当然还是有可能出现从尾部反向遍历找不到的情况，但是不用担心，我们还在 while 循环中，无论如何，节点最后总会入队成功的。最终，transferAfterCancelledWait 都将返回 false。再回到 transferAfterCancelledWait 调用处：
```java
private int checkInterruptWhileWaiting(Node node) {
    return Thread.interrupted() ?
        (transferAfterCancelledWait(node) ? THROW_IE : REINTERRUPT) :
        0;
}
```
在这里，由于 transferAfterCancelledWait 返回了 false，则 checkInterruptWhileWaiting 方法将返回 REINTERRUPT，这说明我们在退出该方法时只需要再次中断。再回到 checkInterruptWhileWaiting 方法的调用处：
```java
public final void await() throws InterruptedException {
    if (Thread.interrupted())
        throw new InterruptedException();
    Node node = addConditionWaiter();
    int savedState = fullyRelease(node);
    int interruptMode = 0;
    while (!isOnSyncQueue(node)) {
        LockSupport.park(this);
        if ((interruptMode = checkInterruptWhileWaiting(node)) != 0)    // 我们在这里！！！
            break;
    }
    // 当前 interruptMode = REINTERRUPT，无论这里是否进入 if 体，该值不变
    if (acquireQueued(node, savedState) && interruptMode != THROW_IE)
        interruptMode = REINTERRUPT;
    if (node.nextWaiter != null) // clean up if cancelled
        unlinkCancelledWaiters();
    if (interruptMode != 0)
        reportInterruptAfterWait(interruptMode);
    }
```
此时，interruptMode 的值为 REINTERRUPT，我们将直接跳出 while 循环。接下来就和上面的情况 1 一样了，我们依然还是去争锁，这一步依然是阻塞式的，获取到锁则退出，获取不到锁则会被挂起。另外由于现在 interruptMode 的值已经为 REINTERRUPT，因此无论在争锁的过程中是否发生过中断 interruptMode 的值都还是 REINTERRUPT。

接着就是将节点从 condition queue 中剔除，与情况 1 不同的是，在 signal 方法成功将 node 加入到 sync queue 时，该节点的 nextWaiter 已经是 null 了，所以这里这一步不需要执行。再接下来就是调用 reportInterruptAfterWait 报告中断状态了。在这种情况下，并不会抛出异常，而只是将当前线程在中断一次。

**情况2.2：中断发生在 acquireQueued 抢占锁的过程中**
这种情况就比上面的情况简单一点了，既然被唤醒时没有发生中断，那基本可以确信线程是被 signal 唤醒的，但是不要忘记还存在 "假唤醒" 这种情况，因此我们依然还是要检测被唤醒的原因。那么怎么区分到底是假唤醒还是因为是被 signal 唤醒了呢？
如果线程是因为 signal 而被唤醒，则由前面分析的 signal 方法可知，线程最终都会离开 condition queue 进入 sync queue 中，所以我们只需要判断被唤醒时，线程是否已经在 sync queue 中即可:
```java
public final void await() throws InterruptedException {
    if (Thread.interrupted())
        throw new InterruptedException();
    Node node = addConditionWaiter();
    int savedState = fullyRelease(node);
    int interruptMode = 0;
    while (!isOnSyncQueue(node)) {
        LockSupport.park(this); // 我们在这里，线程将在这里被唤醒
        // 由于现在没有发生中断，所以 interruptMode 目前为 0
        if ((interruptMode = checkInterruptWhileWaiting(node)) != 0)
            break;
    }
    if (acquireQueued(node, savedState) && interruptMode != THROW_IE)
        interruptMode = REINTERRUPT;
    if (node.nextWaiter != null) // clean up if cancelled
        unlinkCancelledWaiters();
    if (interruptMode != 0)
        reportInterruptAfterWait(interruptMode);
}
```
线程被唤醒时，暂时还没有发生中断，所以这里 interruptMode = 0，表示没有中断发生，所以我们将继续 while 循环，这时我们将通过 isOnSyncQueue 方法判断当前线程是否已经在 sync queue 中了。由于已经发生过 signal 了，则此时 node 必然已经在 sync queue 中了，所以 isOnSyncQueue 将返回 true，直接退出 while 循环。

这里需要注意，如果 isOnSyncQueue 检测到当前节点不在 sync queue 中，则说明既没有发生中断，也没有发生过 signal，则当前线程是被 "假唤醒" 的，那么我们将再次进入循环体，将线程挂起。

退出 while 循环后接下来还是利用 acquireQueued 争锁，因为前面没有发生中断，则 interruptMode = 0，这时，如果在争锁的过程中发生了中断，则 acquireQueued 将返回 true，则此时 interruptMode 将变为 REINTERRUPT。

接下是判断 node.nextWaiter != null，由于在调用 signal 方法时已经将节点移出了队列，所有这个条件也不成立。最后就是汇报中断状态了，此时 interruptMode 的值为 REINTERRUPT，说明线程在被 signal 后又发生了中断，这个中断发生在抢锁的过程中，这个中断来的太晚了，因此我们只是再次自我中断一下。

#### 11、一直没有中断发生
这种情况就更简单了，它的大体流程和上面的情况 2.2 差不多，只是在抢锁的过程中也没有发生异常，则 interruptMode 为 0，没有发生过中断，因此不需要汇报中断。则线程就从 await() 方法处正常返回。

#### 12、await() 总结
1. 进入 await() 时必须是已经持有了锁
2. 离开 await() 时同样必须是已经持有了锁
3. 调用 await() 会使得当前线程被封装成 Node 扔进条件队列，然后释放所持有的锁
4. 释放锁后，当前线程将在 condition queue 中被挂起，等待被 signal 或者中断
5. 线程被唤醒后会将会离开 condition queue 进入 sync queue 中进行抢锁
6. 若在线程抢到锁之前发生过中断，则根据中断发生在 signal 之前还是之后记录中断模式
7. 线程在抢到锁后进行善后工作（离开 condition queue，处理中断异常）
8. 线程已经持有了锁，从 await() 方法返回

#### 13、awaitUninterruptibly()
在前面分析的 await() 方法中，中断起到了和 signal 同样的效果，但是中断属于将一个等待中的线程非正常唤醒，可能即使线程被唤醒后，也抢到了锁，但是却发现当前的等待条件并没有满足，则还是得把线程挂起。因此我们有时候并不希望 await 方法被中断，awaitUninterruptibly() 方法即实现了这个功能：
```java
public final void awaitUninterruptibly() {
    Node node = addConditionWaiter();
    int savedState = fullyRelease(node);
    boolean interrupted = false;
    while (!isOnSyncQueue(node)) {
        LockSupport.park(this);
        if (Thread.interrupted())
            // 发生了中断后只是简单记录中断状态，并继续执行 while 循环
            // 直到当前线程被 signal，由 condition queue 转移到 sync queue
            interrupted = true;
    }
    if (acquireQueued(node, savedState) || interrupted)
        selfInterrupt();
}
```
该方法和前面的 await() 方法相比，可以发现，awaitUninterruptibly() 全程忽略中断，即使是当前线程因为中断被唤醒，该方法也只是简单的记录中断状态，然后再次被挂起（因为并没有并没有任何操作将它添加到 sync queue 中）。
如果要使当前线程离开 condition queue 去争锁，则必须是发生了 signal 事件。
最后，如果线程在获取锁的过程中发生了中断，该方法也是不响应，只是在最终获取到锁返回时，再自我中断一下。可以看出，该方法和 "中断发生于 signal 之后" 的 REINTERRUPT 模式的 await() 方法很像。

> 参考：
  [逐行分析AQS源码(4)——Condition接口实现](https://segmentfault.com/a/1190000016462281)