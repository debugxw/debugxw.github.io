---
title: Java中的线程池
excerpt: Java中管理线程的一种池化技术
date: 2019-04-21 20:31:01
tags: [Java,并发]
image: /img/theme/015.jpg
---

多线程技术主要解决处理器单元内多个线程执行的问题，它可以显著减少处理器单元的闲置时间，增加处理器单元的吞吐能力。假设服务器完成一项任务需要的时间为：创建线程的时间 T1，线程执行任务的时间 T2，销毁线程的时间 T3。如果 T1、T2 之和远大于 T3，那么就可以使用线程池以提高服务器的性能（线程池技术正是关注于如何缩短或调整 T1、T3 的时间，从而提高服务器程序的性能）。线程池的优点：
+ 线程是稀缺资源，使用线程池可以减少创建和销毁线程的次数，每个工作线程都可以重复使用。
+ 可以根据系统的承受能力，调整线程池中工作线程的数量，防止因为消耗过多内存导致服务器崩溃。

ThreadPoolExecutor 是 java 中线程池的核心实现类，该类的构造方法声明如下：
```java
public ThreadPoolExecutor(int corePoolSize,             /* 线程池核心线程数量 */
                    int maximumPoolSize,                /* 线程池最大线程数量 */
                    long keepAliveTime,                 /* 当活跃线程数大于核心线程数时，空闲的多余线程的最大存活时间 */
                    TimeUnit unit,                      /* 存活时间的单位 */
                    BlockingQueue<Runnable> workQueue,  /* 存放任务的列队 */
                    ThreadFactory threadFactory,        /* 创建新线程使用的工厂 */
                    RejectedExecutionHandler handler);  /* 超出线程范围和列队容量的任务的处理程序 */
```

## 一、线程池的主要处理流程
+ 判断正在运行的线程数量是否小于 corePoolSize，如果是，则直接创建一个工作线程来执行任务
+ 判断任务列队是否已满，如果没有，则新提交的任务直接加入到任务列队
+ 判断正在运行的线程数量是否小于 maximumPoolSize，如果是，则直接创建一个工作线程来执行任务；否则，交给饱和策略来处理

ThreadPoolExecutor 中的 executor() 方法如下：
```java
public void execute(Runnable command) {
    if (command == null)
        throw new NullPointerException();
    /*
     * Proceed in 3 steps:
     *
     * 1. If fewer than corePoolSize threads are running, try to
     * start a new thread with the given command as its first
     * task.  The call to addWorker atomically checks runState and
     * workerCount, and so prevents false alarms that would add
     * threads when it shouldn't, by returning false.
     *
     * 2. If a task can be successfully queued, then we still need
     * to double-check whether we should have added a thread
     * (because existing ones died since last checking) or that
     * the pool shut down since entry into this method. So we
     * recheck state and if necessary roll back the enqueuing if
     * stopped, or start a new thread if there are none.
     *
     * 3. If we cannot queue task, then we try to add a new
     * thread.  If it fails, we know we are shut down or saturated
     * and so reject the task.
     */
    int c = ctl.get();
    if (workerCountOf(c) < corePoolSize) {
        if (addWorker(command, true))
            return;
        c = ctl.get();
    }
    if (isRunning(c) && workQueue.offer(command)) {
        int recheck = ctl.get();
        if (! isRunning(recheck) && remove(command))
            reject(command);
        else if (workerCountOf(recheck) == 0)
            addWorker(null, false);
    }
    else if (!addWorker(command, false))
        reject(command);
}
```

## 二、饱和策略
当线程池和任务列队都满了，说明线程池处于一个饱和状态，那么必须对新提交的任务采用一种特殊的策略来处理。ThreadPoolExecutor 为我们提供了四种处理策略：
+ AbortPolicy：直接抛出 RejectedExecutionException 异常（线程池的默认拒绝行为）
+ CallerRunsPolicy：直接由提交任务者执行这个任务
+ DiscardOldestPolicy：丢弃任务列队中最老的一个任务，并尝试为当前提交的任务腾出位置
+ DiscardPolicy：什么也不做，直接忽略

### 测试用例
```java
public class ThreadPoolTest implements Runnable {

    public static void main(String[] args) {
        LinkedBlockingQueue<Runnable> queue =
            new LinkedBlockingQueue<Runnable>(5);
        /* 样例一 */
        ThreadPoolExecutor threadPool =
            new ThreadPoolExecutor(5, 10, 60, TimeUnit.SECONDS, queue);
        /* 样例二
        ThreadPoolExecutor threadPool =
            new ThreadPoolExecutor(5, 10, 60, TimeUnit.SECONDS, queue, new ThreadPoolExecutor.DiscardPolicy());
        */
        for (int i = 0; i < 16; i++) {
            threadPool.execute(
                new Thread(new ThreadPoolTest(), "Thread".concat(i + "")));
            System.out.println("线程池中活跃的线程数： " + threadPool.getPoolSize());
            if (queue.size() > 0) {
                System.out.println("---队列中阻塞的线程数---" + queue.size());
            }
        }
        threadPool.shutdown();
    }

    @Override
    public void run() {
        try {
            Thread.sleep(300);
        } catch (InterruptedException e) {
            e.printStackTrace();
        }
    }
}
```
通过输出结果可以看到，如果使用默认的饱和策略，程序直接抛出异常。而如果使用 DiscardPolicy 策略，程序可以正常退出。

## 三、线程池任务列队
线程池的任务列队用到了 java.util.concurrent 包中的阻塞列队 BlockingQueue，排队通常有三种策略：

### 1、直接提交---SynchronousQueue
Executors.newCachedThreadPool() 返回的 ThreadPoolExecutor 就是使用的这种列队，它将线程直接提交而不保持他们。在此，如果不存在可用于立即运行任务的线程，则试图把任务加入队列将失败，因此会构造一个新的工作线程。此策略可以避免在处理可能具有内部依赖性的请求集时出现锁。直接提交通常要求无界的 maximumPoolSize 以避免拒绝新提交的任务。当任务以超过队列所能处理的平均数连续到达时，此策略允许无界线程具有增长的可能性。

**SynchronousQueue 简介**
SynchronousQueue 内部并没有数据缓存空间，当一个线程向列队中加入一个元素时，如果没有另一个线程来取走这个元素，则线程将进入阻塞状态。由于一个插入操作总是对应一个移除操作，反过来同样需要满足。那么一个元素就不会在 SynchronousQueue 里面长时间停留，一旦有了插入线程和移除线程，元素很快就从插入线程移交给移除线程。也就是说这更像是一种信道（管道），资源从一个方向快速传递到另一方向。显然这是一种快速传递元素的方式，也就是说在这种情况下元素总是以最快的方式从插入者（生产者）传递给移除者（消费者），这在多任务队列中是最快处理任务的方式。

使用示例：
```java
import java.util.concurrent.SynchronousQueue;

public class Main {

    public static void main(String[] args) throws InterruptedException {

        final SynchronousQueue<Integer> queue = new SynchronousQueue<>();

        Thread putThread = new Thread(() -> {
            System.out.println("put thread start");
            try {
                queue.put(1);
            } catch (InterruptedException e) {
            }
            System.out.println("put thread end");
        });

        Thread takeThread = new Thread(() -> {
            System.out.println("take thread start");
            try {
                System.out.println("take from putThread: " + queue.take());
            } catch (InterruptedException e) {
            }
            System.out.println("take thread end");
        });

        putThread.start();
        Thread.sleep(1000);
        takeThread.start();
    }
}
```

### 2、无界列队---LinkedBlockingQueue
Executors.newSingleThreadExecutor 和 Executors.newFixedThreadPool 返回的 ThreadPoolExecutor 就是使用的这种列队。如果当前所有 corePoolSize 线程都正在执行任务，那么新任务将在队列中等待。这里的“无界”是相对于有界列队来说的，因为 LinkedBlockingQueue 中的最大容量为 Integer.MAX_VALUE，或者在计算机资源耗尽的情况下，就无法再添加新的任务了。

### 3、有界列队---ArrayBlockingQueue
这个是最为复杂的使用，所以 JDK 不推荐使用也有些道理。与无界列队相比，它最大的特点就是可以防止资源耗尽的情况发生。
例如：
```java
new ThreadPoolExecutor(
    2, 4, 30, TimeUnit.SECONDS,
    new ArrayBlockingQueue<Runnable>(2),
    new RecorderThreadFactory("CookieRecorderPool"),
    new ThreadPoolExecutor.CallerRunsPolicy());
```
假设，所有的任务都永远无法执行完。对于首先来的 A, B 来说直接运行，接下来，如果来了 C, D，他们会被放到 Queue 中，如果接下来再来 E, F，则增加线程运行 E, F。但是如果再来任务，队列无法再接受了，线程数也到达最大的限制了，所以就会使用拒绝策略来处理。

## 四、Executor 框架的两级调度模型
在 HotSpot VM 的模型中，JAVA 线程被一对一映射为本地操作系统线程。JAVA 线程启动时会创建一个本地操作系统线程，当 JAVA 线程终止时，对应的操作系统线程也被销毁回收，而操作系统会调度所有线程并将它们分配给可用的 CPU。

在上层，JAVA 程序会将应用分解为多个任务，然后使用应用级的调度器（Executor）将这些任务映射成固定数量的线程。在底层，操作系统内核将这些线程映射到硬件处理器上。

Executor 框架类图

<center><img src="../../../../img/jdk/executor.png" width="50%" height="50%" /></center>

在前面介绍的 JAVA 线程既是工作单元，也是执行机制。而在 Executor 框架中，我们将工作单元与执行机制分离开来。Runnable 和 Callable 是工作单元（也就是俗称的任务），而执行机制由 Executor 来提供。这样一来 Executor 是基于生产者消费者模式的，提交任务的操作相当于生成者，执行任务的线程相当于消费者。

在前面我们使用了 ThreadPoolExecutor，但在 jdk 帮助文档中并不推荐直接使用 ThreadPoolExecutor 创建线程池，而是使用 Executors 提供的一些静态方法来创建线程池。常用的一些线程池如下：

### 1、Executors.newSingleThreadExecutor()
```java
public static ExecutorService newSingleThreadExecutor() {
    return new FinalizableDelegatedExecutorService
        (new ThreadPoolExecutor(1, 1,
                                0L, TimeUnit.MILLISECONDS,
                                new LinkedBlockingQueue<Runnable>()));
}
```
从源码中可以知道，单线程线程池的创建也是通过 ThreadPoolExecutor，里面的核心线程数和线程数都是 1，并且工作队列使用的是无界队列。由于是单线程工作，每次只能处理一个任务，所以后面所有的任务都被阻塞在工作队列中，只能一个一个的执行任务。

### 2、Executors.newFixedThreadPool(int nThreads)
```java
public static ExecutorService newFixedThreadPool(int nThreads) {
    return new ThreadPoolExecutor(nThreads, nThreads,
                                  0L, TimeUnit.MILLISECONDS,
                                  new LinkedBlockingQueue<Runnable>());
}
```
与单线程线程池类似，只不过线程池的数量不一样。

### 3、Executors.newCachedThreadPool()
```java
public static ExecutorService newCachedThreadPool() {
    return new ThreadPoolExecutor(0, Integer.MAX_VALUE,
                                  60L, TimeUnit.SECONDS,
                                  new SynchronousQueue<Runnable>());
}
```
该线程池的实现使用到了 SynchronousQueue，当一个任务到达时就执行，线程数量不够就创建，与前面两个的区别是：空闲的线程会被回收掉，空闲的时间上限是 60s。这个适用于执行很多短期异步的小程序或者负载较轻的服务器。

### 4、Executors.newScheduledThreadPool(int corePoolSize)
该方法会返回一个 ScheduledThreadPoolExecutor 类对象，该类继承自 ThreadPoolExecutor，实现了 ScheduledExecutorService 接口。这种线程池可以再给定的延迟后执行任务，也可以定期执行任务。例如：
```java
public class Test {

    public static void main(String[] args) throws InterruptedException {
        ScheduledExecutorService executorService =
            Executors.newScheduledThreadPool(2);
        
        /* 每隔 2s 打印 ===== */
        executorService.scheduleAtFixedRate(() -> {
            System.out.println("=====");
        }, 1000, 2000, TimeUnit.MILLISECONDS);
        
        /* 每隔 4s 打印当前时间 */
        executorService.scheduleAtFixedRate(() -> {
            System.out.println(System.currentTimeMillis());
        }, 1000, 4000, TimeUnit.MILLISECONDS);
    }
}
```


参考：
https://www.oschina.net/question/565065_86540
http://ifeve.com/java-synchronousqueue/
https://www.cnblogs.com/dongguacai/p/6030187.html