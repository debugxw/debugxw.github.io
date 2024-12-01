---
title: JDK1.6之后的锁优化
excerpt: 高效并发是从JDK1.5到JDK1.6的一个重要改进，HotSpot虚拟机开发团队在这个版本上花费了大量的精力去实现各种锁优化技术
date: 2019-04-04 15:55:13
tags: [Java,并发]
image: /img/theme/021.jpg
---

## 一、概述
高效并发是从JDK1.5到JDK1.6的一个重要改进，HotSpot虚拟机开发团队在这个版本上花费了大量的精力去实现各种锁优化技术，如适应性自旋 (Adaptive Spinning) 、锁消除 (Lock Elimination) 、锁粗化 (Lock Coarsening) 、轻量级锁 (Lightweight Locking) 和偏向锁 (Biased Locking) 等，这些技术都是为了在线程之间更高效的共享数据，以及解决竞争问题，从而提高程序的执行效率。

## 二、自旋锁和适应性自旋
互斥同步对性能最大的影响是阻塞的实现，挂起线程和恢复线程的操作都需要转入内核态中完成，这些操作给系统的并发性能带来的很大的压力。而在很多的应用上，共享数据的锁定状态只会持续很短的一段时间，为了这段很短的时间去挂起和恢复线程是很不值得的。所以虚拟机的开发团队就这样考虑：“我们可不可以让后面请求锁的那个线程稍微等一下，但并不放弃处理器的执行时间，看看持有锁的线程是否很快就会释放锁。”为了让线程等待，我们只需让线程执行一个忙循环（自旋），这项技术就是所谓的自旋锁。

自旋锁在 JDK1.4.2 中就已经引入了，只不过是默认关闭的，可以使用 -XX:+UseSpinning 参数来开启，在 JDK1.6 中已经改为默认开启了。自旋锁不能代替阻塞，因为自旋锁虽然避免了线程切换的开销，但它还是要占用处理器时间的，因此，如果锁被占用的时间很短，自旋等待的效果就会非常好，反之，如果锁被占用的时间很长，那么自旋的线程只会白白的消耗处理器资源，而不会做任何有意义的工作，反而会带来性能上的浪费。因此，自旋等待的时间必须要有一定的限度，如果自旋超过的限定的次数任然没有成功获得锁，就应该使用传统的方式去挂起线程。自旋次数的默认值是10次，用户可以使用参数 -XX:PreBlockSpin 来更改。

在 JDK1.6 中引入了自适应的自旋锁。自适应意味着自旋的时间不再固定，而是由前一次在同一个锁上的自旋时间及锁的拥有者的状态来决定。如果在同一个锁对象上，自旋等待刚刚成功获得过锁，并且持有锁的线程正在运行中，那么虚拟机就会认为这一次的自旋很有可能再次成功，进而他允许自旋等待相对更长的时间，例如 100 个循环。另外，如果对于某个锁，自旋很少成功获得过，那么以后在获取这个锁时将可能省略掉自旋过程，以避免浪费处理器资源。

## 三、锁消除
锁消除是指虚拟机即时编译器在运行时，对一些代码上要求同步，但是被检测到不可能存在共享数据竞争的锁进行消除。锁消除的主要判断哪依据来源于逃逸分析的数据支持，如果判断在一段代码中，堆上的所有数据都不会逃逸出去从而被其他线程访问到，那就可以把它们当做栈上数据对待，认为它们是线程私有的，同步枷锁自然就无需进行。（关于逃逸分析技术可参考《深入理解Java虚拟机---JVM高级特性与最佳实战》）

## 四、锁粗化
原则上，我们在编写代码的时候，总是推荐将同步快的作用范围限制的得尽量小，即只在共享数据的实际作用域中才进行同步，这样是为了使得需要同步的操作数量尽可能的小，如果存在竞争，那等待锁的线程也能尽快拿到锁。大部分情况下，这样的原则都是对的，但是如果一系列的连续操作都对同一个对象反复的加锁和解锁，甚至加锁操作出现在循环体中，那即使没有线程竞争，频繁的进行互斥同步操作也会导致不必要的性能消耗。

如果虚拟机探测到有一串零碎的操作都对同一个对象加锁，将会把加锁同步的范围扩展（粗化）到整个操作序列的外部，这样就只需要加锁一次就够了。

## 五、轻量级锁
轻量级锁是 JDK1.6 之中加入的新型锁机制，它名字中的“轻量级”是相对于使用操作系统互斥量来实现的传统锁而言的，因此传统的锁机制就称为“重量级”锁。轻量级锁并不是用来代替重量级锁的，它的本意是在没有多线程竞争的前提下，减少传统的重量级锁使用操作系统互斥量产生的性能消耗。轻量级锁能提升程序同步性能的依据是**对于绝大部分的锁，在整个同步周期内都是不存在竞争的**，这是一个经验数据。如果没有竞争，轻量级锁使用 CAS 操作避免了使用互斥量的开销，但如果存在锁竞争，除了互斥量的开销之外，还额外发生了 CAS 操作，因此在有竞争的情况下，轻量级锁会比传统的重量级锁更慢。

## 六、偏向锁
偏向锁也是 JDK1.6 中引入的一项锁优化，它的目的是**消除数据在无竞争情况下的同步原语，进一步提高程序的运行性能**。如果说轻量级锁实在无竞争的情况下使用 CAS 操作去消除同步使用的互斥量，那偏向锁就是在无竞争的情况下把整个同步都消除掉，连 CAS 操作都不做了。偏向锁的意思是这个锁会偏向于第一个获得他的线程，如果在接下来的执行过程中，该锁没有被其他的线程获取，则持有偏向锁的线程将永远不需要再进行同步。

偏向锁可以提高带有同步但无竞争的程序性能。它同样是一个带有效益权衡性质的优化，它并不一定总是对程序运行有利，如果程序中大多数的锁总是被多个不同的线程访问，那偏向模式就是多余的。在具体问题具体分析的前提下，有时候使用参数 -XX:-UseBiasedLocking 来禁用偏向锁优化反而可以提升性能。

关于轻量级锁和偏向锁理解的不是很透彻，后续还会完善一下。。。

## 七、synchronized VS ReentrantLock
+ **两者都是可重入锁**
两者都是可重入锁，可重入锁是指：自己可以再次获取自己的内部锁。比如一个线程获得了某个对象的锁，此时这个对象锁还没有释放，当其再次想要获取这个对象的锁的时候还是可以获取的，如果不可锁重入的话，就会造成死锁。同一个线程每次获取锁，锁的计数器都自增1，所以要等到锁的计数器下降为0时才能释放锁。
+ synchronized 表现为原生语法 (JVM) 层面的互斥锁，ReentrantLock 则表现为 API 层面的互斥锁（需要 lock() 和 unlock() 方法配合 try/finally 语句块来完成）。
+ 相比 synchronized ，ReentrantLock 增加了一些高级功能，主要有以下三项：**等待可中断**，**可实现公平锁**，**锁可以绑定多个条件（可实现选择性通知）**。
  + 等待可中断是指当持有锁的线程长期不释放锁的时候，正在等待的线程可以选择放弃等待，改为处理其他的事情，可中断特性对处理执行时间非常长的同步快非常有帮助。
```java
/**
 * ReentrantLock为我们提供了一个可以相应中断的获取锁的方法lockInterruptibly()，该方法可以用来解决死锁问题
 *
 * 构造死锁场景：创建两个子线程,子线程在运行时会分别尝试获取两把锁。其中一个线程先获取锁1再获取锁2，另一个线程正好相反。
 * 如果没有外界中断，该程序将处于死锁状态永远无法停止。我们通过使其中一个线程中断，来结束线程间毫无意义的等待。
 * 被中断的线程将抛出异常，而另一个线程将能获取锁后正常结束。
 */
public class ReentrantLockTest {

    static Lock lockOne = new ReentrantLock();

    static Lock lockTwo = new ReentrantLock();

    public static void main(String[] args) {
        Thread threadOne = new Thread(new Task(lockOne, lockTwo));
        Thread threadTwo = new Thread(new Task(lockTwo, lockOne));
        threadOne.start();
        threadTwo.start();
        threadOne.interrupt();//中断第一个线程
    }

    static class Task implements Runnable {
        Lock firstLock;
        Lock secondLock;

        public Task(Lock firstLock, Lock secondLock) {
            this.firstLock = firstLock;
            this.secondLock = secondLock;
        }

        @Override
        public void run() {
            try {
                firstLock.lockInterruptibly();
                TimeUnit.MILLISECONDS.sleep(10);
                secondLock.lockInterruptibly();
            } catch (InterruptedException e) {
                e.printStackTrace();
            } finally {
                firstLock.unlock();
                secondLock.unlock();
                System.out.println(Thread.currentThread().getName() + "正常结束!");
            }
        }
    }
}
```
  + 公平锁是指多个线程在等待同一个锁时，必须按照申请锁的时间顺序来依次获得锁；而非公平锁不保证这一点，任何一个等待锁的线程都有机会获得锁。synchronized 中的锁是非公平的，ReentrantLock 默认情况下也是非公平的，但可以通过带有布尔值的构造函数要求使用公平锁。
```java
public class ReentrantLockTest {

    static Lock lock = new ReentrantLock(true);

    static class Task implements Runnable {
        
        Integer id;

        public Task(Integer id) {
            this.id = id;
        }

        @Override
        public void run() {
            try {
                TimeUnit.MILLISECONDS.sleep(10);
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
            for (int i = 0; i < 2; i++) {
                lock.lock();
                System.out.println("获得锁的线程：" + id);
                lock.unlock();
            }
        }
    }

    public static void main(String[] args) {
        for (int i = 0; i < 5; i++) {
            new Thread(new Task(i)).start();
        }
    }
}
```
  + 锁绑定多个条件是指一个 ReentrantLock 对象可以同时绑定多个 Condition 对象，而在 synchronized 中，锁对象的 wait() 和 notify() 或 notifyAll() 方法可以实现一个隐含的条件，如果要和多于一个的条件关联的时候，就不得不额外的添加一个锁，而 ReentrantLock 无须这样做，只需要多次调用 newCondition() 即可。
```java
/**
 * 使用ReentrantLock和Condition实现简单的阻塞列队
 * 阻塞列队的特点：
 *      1、入队和出队线程安全
 *      2、当列队满时，入队线程会被阻塞；当列队为空时，出队线程会被阻塞
 */
public class MyBlockingQueue<E> {

    private int size;       //阻塞队列最大容量

    private ReentrantLock lock = new ReentrantLock();

    private LinkedList<E> list = new LinkedList<>();      //队列底层实现

    Condition notFull = lock.newCondition();        //队列满时的等待条件
    Condition notEmpty = lock.newCondition();       //队列空时的等待条件

    public MyBlockingQueue(int size) {
        this.size = size;
    }

    public void enqueue(E e) throws InterruptedException {
        lock.lock();
        try {
            while (list.size() == size)     //队列已满, 在notFull条件上等待
                notFull.await();
            list.add(e);        //入队: 加入链表末尾
            System.out.println("入队：" + e);
            notEmpty.signal();      //通知在notEmpty条件上等待的线程
        } finally {
            lock.unlock();
        }
    }

    public E dequeue() throws InterruptedException {
        E e;
        lock.lock();
        try {
            while (list.size() == 0)        //队列为空, 在notEmpty条件上等待
                notEmpty.await();
            e = list.removeFirst();     //出队: 移除链表首元素
            System.out.println("出队：" + e);
            notFull.signal();       //通知在notFull条件上等待的线程
            return e;
        } finally {
            lock.unlock();
        }
    }

    public static void main(String[] args) {
        MyBlockingQueue<Integer> queue = new MyBlockingQueue<>(2);
        for (int i = 0; i < 10; i++) {
            int data = i;
            new Thread(() -> {
                try {
                    queue.enqueue(data);
                } catch (InterruptedException e) {
                    e.printStackTrace();
                }
            }).start();
        }
        for (int i = 0; i < 10; i++) {
            new Thread(() -> {
                try {
                    Integer data = queue.dequeue();
                } catch (InterruptedException e) {
                    e.printStackTrace();
                }
            }).start();
        }
    }
}
```

如果需要使用上诉功能，选用 Reentrantlock 是一个很好的选择。在性能方面，JDK1.6 之前，多线程环境下 synchronized 的吞吐量随着线程数量的增加下降得非常严重，而 ReentrantLock 则能基本保持在一个比较稳定的水平上。与其说 ReentrantLock 性能好，还不如说 synchronized 还有非常大的优化余地。后续的技术发展也证明了这一点，JDK1.6 中加入了很多针对锁的优化措施（如上所诉）。在 JDK1.6 发布之后，synchronized 和 ReentrantLock 的性能基本上是完全持平了。因此性能因素就不再是选择 ReentrantLock 的理由了，虚拟机在未来的性能改进中肯定也会更加偏向于原生的 synchronized，所以还是提倡在 synchronized 能实现需求的情况下，优先考虑使用 synchronized 来进行同步。

> 参考：
 《深入理解 Java 虚拟机：JVM 高级特性与最佳实战》
  https://github.com/Snailclimb/JavaGuide/blob/master/docs/java/synchronized.md
  [ReentrantLock(重入锁)功能详解和应用演示](https://www.cnblogs.com/takumicx/p/9338983.html)

