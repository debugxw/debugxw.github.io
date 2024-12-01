---
title: 浅析Callable、Future与FutureTask
excerpt: 异步编程接口
date: 2019-05-22 17:09:51
tags: 并发
image: /img/theme/007.jpg
---

Callable 与 Future 是在 JAVA 的后续版本中引入进来的，Callable 类似于 Runnable 接口，实现 Callable 接口的类与实现 Runnable 的类都是可以被线程执行的任务。三者之间的关系：
+ Callable 是 Runnable 封装的异步运算任务
+ Future 用来保存 Callable 异步运算的结果
+ FutureTask 封装 Future 的实体类

<center><img src="../../../../img/jdk/futureTask.png" width="50%" height="50%" /></center>

### 一、Callable 与 Runnable 的区别
1. Callable 定义的方法是 call，而 Runnable 定义的方法是 run
2. call 方法有返回值，而 run 方法是没有返回值的
3. call 方法可以抛出异常，而 run 方法不能抛出异常

### 二、Future
Future 表示异步计算的结果，提供了以下方法，主要是判断任务是否完成、取消任务、获取任务执行结果
```java
public interface Future<V> {

    boolean cancel(boolean mayInterruptIfRunning);

    boolean isCancelled();

    boolean isDone();

    V get() throws InterruptedException, ExecutionException;

    V get(long timeout, TimeUnit unit)
        throws InterruptedException, ExecutionException, TimeoutException;
}
```

### 三、FutureTask
可取消的异步计算，此类提供了对 Future 的基本实现，仅在计算完成时才能获取结果，如果计算尚未完成，则阻塞 get() 方法。FutureTask 不仅实现了 Future 接口，还实现了 Runnable 接口，所以不仅可以将 FutureTask 当成一个任务交给 Executor 来执行，还可以通过 Thread 来创建一个线程。

### 四、Callable 与 FutureTask 的使用
```java
import java.util.concurrent.Callable;
import java.util.concurrent.FutureTask;

public class FutureTaskTest {

    public static void main(String[] args) throws InterruptedException {
        Callable<Integer> task = new Task();
        FutureTask<Integer> futureTask = new FutureTask<>(task);
        Thread thread = new Thread(futureTask);
        thread.start();
        Thread.sleep(500);
        futureTask.cancel(true);

        System.out.println("future is cancel: " + futureTask.isCancelled());

        System.out.println("future is done: " + futureTask.isDone());
    }
}

class Task implements Callable<Integer> {

    @Override
    public Integer call() throws Exception {
        System.out.println("Task start!!!");
        Thread.sleep(5000);
        System.out.println("Task end!!!");
        return 10;
    }
}
```
### 五、Callable 与 Future 的使用
```java
import java.util.concurrent.*;

public class FutureTest implements Callable<String> {

    @Override
    public String call() throws Exception {
        System.out.println("task start: " + System.currentTimeMillis() / 1000);
        Thread.sleep(10000);
        return "停电了";
    }

    public static void main(String[] args) throws InterruptedException, ExecutionException {
        Callable<String> task = new FutureTest();
        ExecutorService es = Executors.newSingleThreadExecutor();
        Future<String> future = es.submit(task);
        es.shutdown();
        Thread.sleep(5000);
        System.out.println("主线程休眠5秒: " + System.currentTimeMillis() / 1000);

        String str = future.get();
        System.out.println("Future.get() = " + str + ": " + System.currentTimeMillis() / 1000);
    }
}

/**
 * 输出结果为：
 * task start: 1556095710
 * 主线程休眠5秒: 1556095715
 * Future.get() = 停电了: 1556095720
 */
```
这里的 future 是直接扔到线程池里面去执行的。由于要打印任务的执行结果，所以从执行结果来看，主线程虽然休眠了 5s，但是从 call 方法执行到拿到任务的结果，这中间的时间差正好是 10s，说明 get() 方法会阻塞当前线程直到任务完成。
使用 FutureTask 可以获得同样的效果：
```java
public static void main(String[] args) throws Exception {
    ExecutorService es = Executors.newSingleThreadExecutor();
    Callable<String> task = new FutureTest();
    FutureTask<String> futureTask = new FutureTask<String>(task);
    es.submit(futureTask);
    es.shutdown();
    Thread.sleep(5000);
    System.out.println("主线程等待5秒: " + System.currentTimeMillis() / 1000);
    String str = futureTask.get();
    System.out.println("futureTask.get() = " + str + ": " + System.currentTimeMillis() / 1000);
}
```
以上的组合可以给我们带来这样的一些变化：如有一种场景中，方法 A 返回一个数据需要 10s，A 方法后面的代码运行需要 20s，但是这 20s 的执行过程中，只有后面 10s 依赖于方法 A 执行的结果。如果与以往一样采用同步的方式，势必会有 10s 的时间被浪费，如果采用前面两种组合，则效率会提高：
1. 先把A方法的内容放到 Callable 实现类的 call() 方法中
2. 在主线程中通过线程池执行 A 任务
3. 执行主线程方法中 10 秒不依赖方法 A 运行结果的代码
4. 获取方法 A 的运行结果，执行后面方法中 10 秒依赖方法 A 运行结果的代码

这样程序的执行效率一下子就提高了，程序不必卡在 A 方法处。

### 六、FutureTask 源码分析
Future 只是一个接口，无法直接创建对象，因此有了 FutureTask。FutureTask 实现了RunnableFuture 接口，而 RunnableFuture 接口继承了 Runnable 和 Future 接口。FutureTask 是一个可取消的异步计算，FutureTask 实现了 Future 的基本方法，提供了 start、cancel 操作，可以查询计算是否已经完成，并且可以获取计算的结果。结果只可以在计算完成之后获取，当计算没有完成的时候 get() 方法会被阻塞，一旦计算已经完成， 那么计算就不能再次启动或是取消。一个 FutureTask 可以用来包装一个 Callable 或是一个 Runnable 对象。因为 FurtureTask 实现了 Runnable 方法，所以一个 FutureTask 可以提交给一个 Excutor 执行。它同时实现了 Future，所以也可以作为 Future 得到 Callable 的返回值。

FutureTask 有两个很重要的属性分别是 state 和 runner，FutureTask 之所以支持 canacel 操作，也是因为这两个属性。
```java
    /**
     * 任务的状态在初始化时为 NEW，只有在方法 set、setException 或者 cancel 方法中才可以转换为最终态。
     * 在任务完成期间，state 的值可能为 COMPLETING 或者 INTERRUPTING。
     *
     * state 有可能的状态转换：
     * NEW -> COMPLETING -> NORMAL
     * NEW -> COMPLETING -> EXCEPTIONAL
     * NEW -> CANCELLED
     * NEW -> INTERRUPTING -> INTERRUPTED
     */
    private volatile int state;
    private static final int NEW          = 0;  // 任务新建或执行中
    private static final int COMPLETING   = 1;  // 任务将要执行完毕
    private static final int NORMAL       = 2;  // 任务正常执行结束
    private static final int EXCEPTIONAL  = 3;  // 任务异常
    private static final int CANCELLED    = 4;  // 任务取消
    private static final int INTERRUPTING = 5;  // 任务线程即将被中断
    private static final int INTERRUPTED  = 6;  // 任务线程已中断
    
    /** The underlying callable; nulled out after running */
    private Callable<V> callable;
    /** The result to return or exception to throw from get() */
    private Object outcome; // 不用加 volatile，因为 state 保证了它的线程安全
    /** The thread running the callable; CASed during run() */
    private volatile Thread runner;
    /** Treiber stack of waiting threads */
    private volatile WaitNode waiters;  // 用于存放阻塞在 FutureTask.get() 方法的线程
```

**构造器**
```java
    public FutureTask(Callable<V> callable) {
        if (callable == null)
            throw new NullPointerException();
        this.callable = callable;
        this.state = NEW;       // ensure visibility of callable
    }
    
    public FutureTask(Runnable runnable, V result) {
        this.callable = Executors.callable(runnable, result);
        this.state = NEW;       // ensure visibility of callable
    }
```
FutureTask 有两个构造器，其中第一个构造器接受一个 Callable 参数并将其赋值给 FutureTask 的内部成员 callable，然后把 state 设置为 NEW。而第二个构造器接受一个 Runnable 参数，然后将其转换为 Callable。Excutors.callable() 方法和 RunnableAdapter 类源码如下。可以看到，实际上是将 Runnable 转化为了 Callable，Callable 的 call 方法实际上调用的是 Runnable 的 run 方法，并把传入的参数 result 作为 Callable 的返回结果。
```java
    public static <T> Callable<T> callable(Runnable task, T result) {
        if (task == null)
            throw new NullPointerException();
        return new RunnableAdapter<T>(task, result);
    }
```
```java
    static final class RunnableAdapter<T> implements Callable<T> {
        final Runnable task;
        final T result;
        RunnableAdapter(Runnable task, T result) {
            this.task = task;
            this.result = result;
        }
        public T call() {
            task.run();
            return result;
        }
    }
```

**run() 方法**
当创建完一个 Task，通常会提交给 Excutors 来执行，或者使用 Thread 来执行，不管使用哪种方式，执行任务都会调用 Task 的 run() 方法。FutureTask 中的 run() 方法实现如下：
```java
    public void run() {
        if (state != NEW || !UNSAFE.compareAndSwapObject(this, runnerOffset, null, Thread.currentThread()))
            return;
        try {
            Callable<V> c = callable;
            // 这里再次判断state是否为NEW，是为了检测在上面的if语句到该条if语句之间是否有其它线程改变了state
            // 例如，在这期间另外一个线程调用了cancel()方法取消任务
            if (c != null && state == NEW) {
                V result;
                boolean ran;
                try {
                    result = c.call();
                    ran = true;
                } catch (Throwable ex) {
                    result = null;
                    ran = false;
                    setException(ex);
                }
                if (ran)
                    set(result);
            }
        } finally {
            // runner must be non-null until state is settled to
            // prevent concurrent calls to run()
            runner = null;
            // state must be re-read after nulling runner to prevent
            // leaked interrupts
            int s = state;
            if (s >= INTERRUPTING)
                handlePossibleCancellationInterrupt(s);
        }
    }
```
大致流程：
+ 首先判断 state 是否为 NEW，如果不为 NEW 则说明已执行过或者已被取消，直接返回
+ 如果状态为 NEW，则使用 Unsafe 类的 CAS 操作将 runner 赋值为 Thread.currentThread()，如果赋值失败，直接返回（[关于 CAS](https://debugxw.github.io/2019/04/26/Compare-and-Swap/)）
+ 执行任务
+ 如果执行任务发生异常，则调用 setException() 方法保存异常信息
+ 如果任务执行成功，则调用 set() 方法设置结果

**setException() 和 set() 方法**
```java
    protected void setException(Throwable t) {
        if (UNSAFE.compareAndSwapInt(this, stateOffset, NEW, COMPLETING)) {
            outcome = t;
            UNSAFE.putOrderedInt(this, stateOffset, EXCEPTIONAL); // final state
            finishCompletion();
        }
    }
```
```java
    protected void set(V v) {
        if (UNSAFE.compareAndSwapInt(this, stateOffset, NEW, COMPLETING)) {
            outcome = v;
            UNSAFE.putOrderedInt(this, stateOffset, NORMAL); // final state
            finishCompletion();
        }
    }
```
setException() 方法和 set() 方法类似，大致流程为：
+ 使用 CAS 操作将 state 设置为 COMPLETING，如果失败，直接退出
+ 将结果（异常信息或任务执行成功正常的返回结果）赋值给 outcome
+ 使用 CAS 操作将 state 设置为 EXCEPTIONAL 或者 NORMAL
+ 调用 finishCompletion() 方法，唤醒 waiters 列队中的线程并执行相关操作（finishCompletion() 方法见后文）

**get() 方法**
```java
    public V get() throws InterruptedException, ExecutionException {
        int s = state;
        if (s <= COMPLETING)
            s = awaitDone(false, 0L);
        return report(s);
    }
```
任务发起线程可以调用 get() 方法来获取任务执行结果。如果此时任务已经执行完毕则调用 report() 方法直接返回结果，如果任务还没有执行完毕则调用 awaitDone() 阻塞等待，知道任务执行完毕。

**awaitDone() 方法**
```java
    private int awaitDone(boolean timed, long nanos)
        throws InterruptedException {
        // 计算等待截止时间
        final long deadline = timed ? System.nanoTime() + nanos : 0L;
        WaitNode q = null;
        boolean queued = false;
        for (;;) {
            // 判断被阻塞线程是否被中断，如果是，则在等待列队中删除该节点并抛出InterruptedException异常
            if (Thread.interrupted()) {
                removeWaiter(q);
                throw new InterruptedException();
            }

            int s = state;
            // 获取当前状态，如果状态大于COMPLETING，说明任务已经结束，要么正常结束，要么异常结束，要么被取消
            // 并把thread显示置空，然后返回结果
            if (s > COMPLETING) {
                if (q != null)
                    q.thread = null;
                return s;
            }
            // 如果状态处于中间状态COMPLETING，表示任务已经结束，但是还没来得及给outcome赋值
            // 这个时候让出执行权让其它线程优先执行
            else if (s == COMPLETING) // cannot time out yet
                Thread.yield();
            // 如果等待节点为空，则构造一个等待节点
            else if (q == null)
                q = new WaitNode();
            // 如果当前节点还没有入队，则当前节点以头插法的方式入队，并将其赋值给waiters
            else if (!queued)
                queued = UNSAFE.compareAndSwapObject(this, waitersOffset,
                                                     q.next = waiters, q);
            else if (timed) {
                // 如果需要等待特定时间，则先计算要等待的时间，如果已超时，则删除对应节点并返回对应的状态
                nanos = deadline - System.nanoTime();
                if (nanos <= 0L) {
                    removeWaiter(q);
                    return state;
                }
                // 阻塞等待特定的时间
                LockSupport.parkNanos(this, nanos);
            }
            else
                // 阻塞等待直到被其他线程唤醒
                LockSupport.park(this);
        }
    }
```
awaitDone() 中有个死循环，每一次循环的大致流程为：
+ 判断调用 get() 的线程是否被其他线程中断，如果是的话则在等待队列中删除对应节点然后抛出 InterruptedException 异常
+ 获取任务当前状态，如果当前任务状态大于 COMPLETING 则表示任务执行完成，则把 thread 字段置 null 并返回结果
+ 如果任务处于 COMPLETING 状态，则表示任务已经处理完成（正常执行完成或者执行出现异常），但是执行结果或者异常原因还没有保存到 outcome 字段中。这个时候调用线程让出执行权让其他线程优先执行
+ 如果等待节点为空，则构造一个等待节点 WaitNode
+ 如果上一步中新建的节点还没如队列，则使用 CAS 操作把该节点加入 waiters 队列的首节点
+ 使用 LockSupport 阻塞当前线程（[更多关于 LockSupport](https://debugxw.github.io/2019/05/30/LockSupport/)）

**report() 方法**
```java
    /* report() 方法用在 get() 中，作用是把不同的任务状态映射成任务执行结果 */
    private V report(int s) throws ExecutionException {
        Object x = outcome;
        // 任务正常执行完成，直接返回任务执行结果
        if (s == NORMAL)
            return (V)x;
        // 如果任务被取消，抛出CancellationException异常
        if (s >= CANCELLED)
            throw new CancellationException();
        // 其它情况，抛出执行异常ExecutionException
        throw new ExecutionException((Throwable)x);
    }
```

**cancel() 方法**
```java
    public boolean cancel(boolean mayInterruptIfRunning) {
        if (!(state == NEW &&
              UNSAFE.compareAndSwapInt(this, stateOffset, NEW,
                  mayInterruptIfRunning ? INTERRUPTING : CANCELLED)))
            return false;
        try {    // in case call to interrupt throws exception
            if (mayInterruptIfRunning) {
                try {
                    Thread t = runner;
                    if (t != null)
                        t.interrupt();
                } finally { // final state
                    UNSAFE.putOrderedInt(this, stateOffset, INTERRUPTED);
                }
            }
        } finally {
            finishCompletion();
        }
        return true;
    }
```
cancel() 方法大致流程如下：
+ 如果 state 不为 NEW，说明任务已经执行结束；或者在使用 CAS 操作重新设置 state 失败，在这两种情况下，直接返回 false
+ 如果参数 mayInterruptIfRunning 为 true，中断执行线程，然后将 state 设置成 INTERRUPTED
+ 最后执行 finishCompletion() 方法

**finishCompletion() 方法**
```java
    private void finishCompletion() {
        // assert state > COMPLETING;
        for (WaitNode q; (q = waiters) != null;) {
            if (UNSAFE.compareAndSwapObject(this, waitersOffset, q, null)) {
                for (;;) {
                    Thread t = q.thread;
                    if (t != null) {
                        q.thread = null;
                        LockSupport.unpark(t);
                    }
                    WaitNode next = q.next;
                    if (next == null)
                        break;
                    q.next = null; // unlink to help gc
                    q = next;
                }
                break;
            }
        }

        done();

        callable = null;        // to reduce footprint
    }
```
不管是任务执行异常还是任务正常执行完毕，或者任务取消，最后都会调用 finishCompletion() 放法。该方法的实现比较简单，依次遍历 waiters 链表，唤醒节点中的线程，然后把 callable 置空。被唤醒的线程会各自从 awaitDone() 方法中的 LockSupport.park*() 阻塞中返回，然后会进行新一轮的循环，在新一轮的循环中会返回执行结果。

参考：
https://www.cnblogs.com/dongguacai/p/6030187.html
https://www.cnblogs.com/linghu-java/p/8991824.html
https://blog.csdn.net/songmaolin_csdn/article/details/78839835

