---
title: LockSupport
excerpt: 用于创建锁和其他同步类的基本线程阻塞原语
date: 2019-05-30 10:44:45
tags: [Java,并发]
image: /img/theme/012.jpg
---

### 一、使用
在没有 LockSupport 之前，线程的挂起和唤醒通常是通过 wait 和 notify 来实现，并且 wait 和 notify 只能在 synchronized 同步语句块中调用。例如：
```java
public class WaitAndNotify {

    public static void main(String[] args) throws InterruptedException {
        final Object object = new Object();
        new Thread(() -> {
            System.out.println("minor thread start!");
            synchronized (object) {
                try {
                    object.wait();
                } catch (InterruptedException e) {
                    e.printStackTrace();
                }
            }
            System.out.println("minor thread end!");
        }).start();

        Thread.sleep(1000);
        synchronized (object) {
            System.out.println("minor thread wake up by main thread!");
            object.notify();
        }
    }
}

/*
    minor thread start!
    minor thread wake up by main thread!
    minor thread end!
 */
```
wait 和 notify 不仅要保证在 synchronized 同步语句块中使用，还要保证使用的顺序，即先 wait 再 notify，否则被挂起的线程永远不会被唤醒。而使用 LockSupport 就相对简单得多，只需调用 park() 和 unpark() 方法，而且不用在意两者的调用顺序。例如：
```java
import java.util.concurrent.locks.LockSupport;

public class LockSupportTest {

    public static void main(String[] args) throws InterruptedException {
        Thread minor = new Thread(() -> {
            System.out.println("minor thread start!");
            LockSupport.park();
            System.out.println("minor thread end!");
        });
        minor.start();

        System.out.println("minor thread wake up by main thread!");
        LockSupport.unpark(minor);
    }
}

/*
    minor thread wake up by main thread!
    minor thread start!
    minor thread end!
 */
```

### 二、解析
从上面的例子中可以看到，LockSupport 通过使用 pack 挂起线程，unpark 唤醒线程。在 LockSupport 中有一个许可（permit）的概念，LockSupport 也是通过这个许可来实现线程的挂起和唤醒。可以简单地按照如下逻辑理解：
+ 调用 park() 时，如果线程的 permit 存在，那么该 permit 被消耗，线程不会被挂起，park() 方法立即返回。如果 permit 不存在，则认为线程缺少 permit，挂起线程等待 permit。（一个线程默认是没有 permit 的）
+ 调用 unpark() 时，如果线程的 permit 不存在，则释放一个 permit，因为有了 permit，所以如果线程处于挂起状态，那么此线程会被线程调度器唤醒。如果线程的 permit 存在，permit 不会累加，看起来就像什么都没做一样。正因为 permit 不能累加，相当于只有一个，所以 LockSupport 是不可重入的，而且调用 park() 和 unpark() 的先后顺序也就不那么重要了。

### 三、源码
#### parkBlockerOffset
```java
    private static final sun.misc.Unsafe UNSAFE;
    private static final long parkBlockerOffset;
    ···
    static {
        try {
            UNSAFE = sun.misc.Unsafe.getUnsafe();
            Class<?> tk = Thread.class;
            parkBlockerOffset = UNSAFE.objectFieldOffset
                (tk.getDeclaredField("parkBlocker"));
            ···
        } catch (Exception ex) { throw new Error(ex); }
    }
```
在 Thread 类中有一个成员变量 parkBlocker，这个 parkBlocker 在线程因为使用 LockSupport 被阻塞时会被记录下来，这样监视工具和诊断工具就可以确定线程受阻塞的原因。这个 parkBlocker 也只能在 LockSupport 中被设置和访问。从上面的静态语句块中可以看到，初始化 parkBlockerOffset 时，会先通过反射获得 Thread 类的 parkBlocker 字段，然后通过 Unsafe 对象的 objectFieldOffset 方法获取到 parkBlocker 相对于 Thread 对象在内存中的起始地址的偏移量。

#### park()
```java
    public static void park(Object blocker) {
        // 获取当前线程
        Thread t = Thread.currentThread();
        // 设置当前线程的 blocker 对象
        setBlocker(t, blocker);
        // 通过Unsafe类挂起线程
        UNSAFE.park(false, 0L);
        // 挂起线程被唤醒后，需要将blocker清理掉
        setBlocker(t, null);
    }
    
    /* 其他重载版本 */
    // 和 park(Object) 相比少了挂起前为线程设置 blocker、被唤醒后清理 blocker 的操作
    public static void park() {
        UNSAFE.park(false, 0L);
    }
    
    // 指定一个挂起时间（相对于当前时间，单位为纳秒），时间到后自动被唤醒
    public static void parkNanos(Object blocker, long nanos) {
        if (nanos > 0) {
            Thread t = Thread.currentThread();
            setBlocker(t, blocker);
            UNSAFE.park(false, nanos);
            setBlocker(t, null);
        }
    }
    
    // 相对于 parkNanos(Object blocker, long nanos)，仅仅少了 blocker 相关操作
    public static void parkNanos(long nanos) {
        if (nanos > 0)
            UNSAFE.park(false, nanos);
    }
    
    // 指定一个挂起时间（绝对时间，单位为毫秒），时间到后自动被唤醒
    public static void parkUntil(Object blocker, long deadline) {
        Thread t = Thread.currentThread();
        setBlocker(t, blocker);
        UNSAFE.park(true, deadline);
        setBlocker(t, null);
    }
    
    // 相对于 parkUntil(Object blocker, long deadline)，仅仅少了 blocker 相关操作
    public static void parkUntil(long deadline) {
        UNSAFE.park(true, deadline);
    }
```
park 主要功能：如果 permit 存在，那么将这个 permit 消耗掉，并且立即返回。如果许可不存在，那么挂起当前线程，直到以下任意事件发生：
+ 其它线程以当前线程为参数，调用 unpark(Thread) 方法
+ 其它线程通过 Thread.interrupt() 中断当前线程
+ 该方法毫无理由的返回

关于上面提到的最后一点，在 jdk 源码中的解释如下：
```java
    /**
     * ··· ···
     * <li>The call spuriously (that is, for no reason) returns.
     * </ul>
     *
     * <p>This method does <em>not</em> report which of these caused the
     * method to return. Callers should re-check the conditions which caused
     * the thread to park in the first place. Callers may also determine,
     * for example, the interrupt status of the thread upon return.
     */
```

#### unpark
```java
    public static void unpark(Thread thread) {
        if (thread != null)
            UNSAFE.unpark(thread);
    }
```
功能：设置线程许可可用。如果当前线程已经被 park 挂起，那么这个线程会被唤醒。如果线程没有被挂起，那么线程下次调用 park 不会被挂起。

### 四、Java docs 中的示例用法：一个先进先出非重入锁框架
```java
class FIFOMutex {
    private final AtomicBoolean locked = new AtomicBoolean(false);
    private final Queue<Thread> waiters
        = new ConcurrentLinkedQueue<Thread>();

    public void lock() {
        boolean wasInterrupted = false;
        Thread current = Thread.currentThread();
        waiters.add(current);

        // Block while not first in queue or cannot acquire lock
        while (waiters.peek() != current ||
            !locked.compareAndSet(false, true)) {
            LockSupport.park(this);
            if (Thread.interrupted()) // ignore interrupts while waiting
                wasInterrupted = true;
        }

        waiters.remove();
        if (wasInterrupted)          // reassert interrupt status on exit
            current.interrupt();
    }

    public void unlock() {
        locked.set(false);
        LockSupport.unpark(waiters.peek());
    }
}
```
测试样例：
```java
public class Test {

    public static void main(String[] args) throws InterruptedException {
        FIFOMutex mutex = new FIFOMutex();
        MyThread a1 = new MyThread("a1", mutex);
        MyThread a2 = new MyThread("a2", mutex);
        MyThread a3 = new MyThread("a3", mutex);

        a1.start();
        a2.start();
        a3.start();

        a1.join();
        a2.join();
        a3.join();

        System.out.print("Finished");
    }
}

class MyThread extends Thread {
    private String name;
    private FIFOMutex mutex;
    public static int count;

    public MyThread(String name, FIFOMutex mutex) {
        this.name = name;
        this.mutex = mutex;
    }

    @Override
    public void run() {
        for (int i = 0; i < 100; i++) {
            mutex.lock();
            count++;
            System.out.println("Thread: " + name + "-----count = " + count);
            mutex.unlock();
        }
    }
}
```

### 五、比较
LockSupport 中的 park 和 unpark、Object 中的 wait 和 notify 都可以实现线程的挂起与唤醒。当使用 wait 和 notify 时，它并不是直接对线程操作，而需要一个 Object 来进行线程的挂起和唤醒。在调用 wait 前，当前线程必须先获得该对象的监视器，被唤醒后需要重新获得该对象的监视器才能继续执行。而 LockSupport 直接面向 Thread，并对其进行挂起和唤醒，语义更加清晰，使用起来也更加方便。

LockSupport 还可以避免出现 Thread.suspend 和 Thread.resume 的死锁问题。Thread.suspend 和 Thread.resume 是不推荐使用的，[官方的解释为](https://docs.oracle.com/javase/6/docs/technotes/guides/concurrency/threadPrimitiveDeprecation.html)：Thread.suspend 本质上是容易发生死锁的，因为它在挂起一个线程时并不会释放线程所占用的资源。如果目标线程在某个系统资源的监视器上持有锁，那么在调用 Thread.resume 恢复目标线程之前，其它任何线程都无法访问这个资源。如果恢复目标线程的线程在调用 Thread.resume 之前尝试锁定该系统资源的监视器，那么就会发生死锁。网上的很多文章都说 LockSupport 可以解决上诉问题，但是 LockSupport 中的 park 挂起线程时，也不会释放线程所占的资源。如下程序，将会发生死锁，因为在主线程调用 unpark 之前尝试获取 Resource 类对象的锁，而这时 minor 线程虽然被挂起，但并未释放所占资源，导致主线程获取锁失败，也被挂起，这样就会导致死锁（不知道是不是自己的理解问题，如下的程序并不能解决死锁问题）：
```java
import java.util.concurrent.locks.LockSupport;

public class Test {

    public static void main(String[] args) throws InterruptedException {
        Thread thread = new Thread(() -> {
            synchronized (Resource.class) {
                Resource.print();
                LockSupport.park();
            }
        }, "minor");
        thread.start();

        Thread.sleep(100);

        synchronized (Resource.class) {
            Resource.print();
            LockSupport.unpark(thread);
        }
        System.out.println("end");
    }
}

class Resource {
    public static void print() {
        System.out.println(Thread.currentThread().getName() + "---print");
    }
}
```
而如果将上面的 LockSupport 改为 wait 和 notify，则不会发生死锁，因为调用 wait 导致线程被挂起时，线程会释放所占资源，这样主线程就可获取锁，然后唤醒 minor 线程：
```java
public class Test {

    public static void main(String[] args) throws InterruptedException {
        Thread thread = new Thread(() -> {
            synchronized (Resource.class) {
                Resource.print();
                try {
                    Resource.class.wait();
                } catch (InterruptedException e) {
                    e.printStackTrace();
                }
            }
        }, "minor");
        thread.start();

        Thread.sleep(100);

        synchronized (Resource.class) {
            Resource.print();
            Resource.class.notify();
        }
        System.out.println("end");
    }
}

class Resource {
    public static void print() {
        System.out.println(Thread.currentThread().getName() + "---print");
    }
}

/*
    输出结果：
        minor---print
        main---print
        end
 */
```
但是 LockSupport 却可以解决 Thread.suspend 的另一个问题，那就是如果在目标线程 Thread.suspend 发生之前调用 Thread.resume，那么目标线程如果没有其他的外界因素，将会一直被阻塞下去。wait 和 notify 也存在这样的问题，但是在 LockSupport 中，由于 permit 的存在，以及 park 和 unpark 的机制，park 和 unpark 的调用顺序就显得不那么重要了，即使先调用 unpark 再调用 park，程序依然可以正确运行，如下：
```java
import java.util.concurrent.locks.LockSupport;

public class Test {

    public static void main(String[] args) {
        System.out.println("start!");
        LockSupport.unpark(Thread.currentThread());
        LockSupport.park();
        System.out.println("end!");
    }
}
```
**注意：如果线程是新建态（即刚新建了一个线程对象，还没有调用 start() 将它变为可运行态），这个时候调用 park 或 unpark 将不起作用。**这里最主要针对 unpark，因为 park 一定是在线程运行过程中调用，而 unpark 可能会在线程还是新建态时就调用了。如下程序，minor 线程将会永远被阻塞下去：
```java
import java.util.concurrent.locks.LockSupport;

public class Main {

    public static void main(String[] args) {
        Thread minor = new Thread(() -> {
            LockSupport.park();
        }, "minor");

        LockSupport.unpark(minor);
        minor.start();
    }
}
```

> 参考
[LockSupport](https://yq.aliyun.com/articles/493552)
[Java多线程进阶-LockSupport](https://segmentfault.com/a/1190000015562456)