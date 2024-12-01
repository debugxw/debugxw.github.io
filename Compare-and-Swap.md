---
title: Compare-and-Swap
excerpt: 通过硬件指令实现的一种并发控制算法，用于在多线程编程环境下确保共享变量的原子性操作
date: 2019-04-26 20:58:01
tags: 并发
image: /img/theme/020.jpg
---

在并发编程中，互斥同步最主要的问题就是进行线程阻塞和唤醒所带来的性能问题，因此这种同步也称为阻塞同步 (Blocking Synchronization)。从处理问题的方式上说，互斥同步属于一种悲观的并发策略，总是认为只要不去做正确的同步策略（例如加锁），那就肯定会出现问题，无论共享数据是否真的会出现竞争，它都要进行加锁（这里讨论的是概念模型，实际上 JVM 会优化掉很大一部分不必要的加锁）、用户态和核态转换、维护锁计数器和检查是否有被阻塞的线程需要唤醒等操作。随着硬件指令集的发展，我们有了另一个选择：基于冲突检测的乐观并发策略，通俗的说，就是先进行操作，如果没有其他线程争用共享数据，那么操作就成功了；如果共享数据有争用，产生了冲突，那就再采取其他的补偿措施（最常见的补偿措施就是不断的重试，直到成功为止），这种乐观的并发策略的许多实现都不需要把线程挂起，因此这种同步操作称为非阻塞同步（Non-Blocking Synchronization）。

为什么说使用乐观并发策略需要“硬件指令集的发展”才能进行呢？因为我们需要操作和冲突检测这两个步骤具备原子性，靠什么来保证呢？如果这里再使用互斥同步来保证就失去意义了，所以我们只能靠硬件来完成这件事情，硬件保证一个从语义上看起来需要多次操作的行为只通过一条处理器指令就能完成，这类指令常用的有：
+ 测试并设置（Test-and-Set）
+ 获取并增加（Fetch-and-Increment）
+ 交换（Swap）
+ 比较并交换（Compare-and-Swap, CAS）
+ 加载链接（Load-Linked/Store-Conditional, LL/SC）

其中，前面三条是 20 世纪就已经存在于大多数指令集之中的处理器指令，后面的两条是现代处理器新增的，而且这两条指令的目的和功能是类似的。在 IA64、x86 指令集中有 cmpxchg 指令完成 CAS 功能，在 sparc-TSO 也有 casa 指令实现，而在 ARM 和 PowerPC 架构下，则需要使用一对 ldrex/strex 指令来完成 LL/SC 功能。

## CAS操作
CAS 指令需要有三个操作数，分别是内存位置（在 Java 中可以简单地理解为变量的内存地址，用 V 表示）、旧的预期值（用 A 表示）和新值（用 B 表示）。CAS 指令执行时，当且仅当 V 符合旧预期值 A 时，处理器用新值 B 更新 V 的值，否则他就不执行更新，但是无论是否更新了 V 的值，都会返回 V 的旧值，上述的操作过程是一个原子操作。

在 JDK1.5 之后，Java 程序中才能使用 CAS 操作，该操作由 sun.misc.Unsafe 类里面的 compareAndSwapInt() 和 compareAndSwapLong() 等几个方法包装提供，虚拟机在内部对这些方法做了特殊处理，即时编译出来的结果就是一条平台相关的处理器 CAS 指令，没有方法调用的过程，或者可以认为是无条件内联进去了（这种被虚拟机特殊处理的方法称为固有函数（Intrinsics），类似的固有函数还有 Math.sin() 等）。由于 Unsafe 类不是提供给用户程序调用的类（Unsafe.getUnfase() 的代码中限制了只有启动类加载器（Bootstrap ClassLoader）加载的 Class 才能访问它），因此，如果不采用反射手段，我们只能通过其他的 Java API 来间接使用它，如 J.U.C 包里面的整数原子类，其中的 compareAndSet() 和 getAndIncrement() 等方法都是使用了 Unsafe 类的 CAS 操作。下面使用 Unsafe 类实现一个简单的 AtomicInteger：
```java
import sun.misc.Unsafe;

import java.lang.reflect.Field;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class UnsafeTest {

    public static void main(String[] args) throws Exception {
        MyInteger myInteger = new MyInteger(0);
        ExecutorService service = Executors.newFixedThreadPool(20);
        for (int cnt = 0; cnt < 20; cnt++) {
            service.execute(() -> {
                for (int i = 0; i < 10000; i++)
                    myInteger.increaseAndGet();
            });
        }
        service.shutdown();
        /* 等待线程池中的所有线程执行完毕 */
        while (Thread.activeCount() > 2)
            Thread.yield();
        System.out.println(myInteger.getValue());
    }
}

class MyInteger {

    private int value;

    private static Unsafe unsafe;

    /* 表示 value 相对于 MyInteger 实例起始地址的偏移量 */
    private static long valueOffset;

    public MyInteger(int value) throws Exception {
        this.value = value;

        /* 使用反射才能获得 Unsafe 实例 */
        Field field = Unsafe.class.getDeclaredField("theUnsafe");
        field.setAccessible(true);
        unsafe = (Unsafe) field.get(Unsafe.class);

        field = MyInteger.class.getDeclaredField("value");
        valueOffset = unsafe.objectFieldOffset(field);
    }

    public int increaseAndGet() {
        while (true) {
            int current = value;
            int next = value + 1;
            if (unsafe.compareAndSwapInt(this, valueOffset, current, next) == true)
                return next;
        }
    }

    public int getValue() {
        return value;
    }
}
```
其中，increaseAndGet() 方法在一个无限循环当中，不断尝试将一个比当前值大一的新值赋值给自己。如果失败，那说明在执行 “获取-设置” 操作期间有其他线程修改 value 的值，于是再次循环进行下一次操作，直到设置成功为止。

## ABA问题
尽管 CAS 看起来很棒，但显然这种操作无法涵盖互斥同步的所有使用场景，并且 CAS 从语义上来说并不是完美的，存在这样一个逻辑漏洞：如果一个变量 V 初次读取的时候是 A 值，并且在准备赋值的时候检查到它任然为 A 值，那我们就能说它的值没有被其他线程改变过了吗？如果在这期间它的值曾经被改成了 B，后来又被改回为 A，那 CAS 操作就会误认为变量 V 从来没有被改变过。这个漏洞称为 CAS 操作的 ABA 问题。J.U.C 包为了解决这个问题，提供了一个带有标记的原子引用类 AtomicStampedReference，他可以通过控制变量值的版本来保证 CAS 的正确性。不过目前来说这个类比较鸡肋，大部分情况下 ABA 问题不会影响程序并发的正确性，如果需要解决 ABA 问题，改用传统的互斥同步可能会比原子类更加高效。ABA 问题样例：
```java
import java.util.concurrent.atomic.AtomicInteger;

public class CAS_ABA_Q {

    public static AtomicInteger a = new AtomicInteger(1);

    public static void main(String[] args){
        Thread main = new Thread(() -> {
            System.out.println(Thread.currentThread().getName() + ",初始值 = " + a);
            try {
                Thread.sleep(1000);     //等待1秒 ，以便让干扰线程执行
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
            boolean isCASSuccess = a.compareAndSet(1, 2);
            System.out.println(Thread.currentThread().getName() + "---CAS操作结果: " + isCASSuccess);
        }, "主操作线程");

        Thread other = new Thread(() -> {
            try {       //确保让main线程优先执行
                Thread.sleep(500);
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
            a.incrementAndGet();
            System.out.println(Thread.currentThread().getName() + ", [increment], a = " + a);
            a.decrementAndGet();
            System.out.println(Thread.currentThread().getName() + ", [decrement], a = " + a);
        }, "干扰线程");

        main.start();
        other.start();
    }
}
```

## 如何解决

### AtomicStampedReference

AtomicStampedReference 原子类是一个带有类似于版本号的对象引用，在每次修改后，AtomicStampedReference 不仅会设置新值而且还会记录更改的版本号。当 AtomicStampedReference 设置对象值时，对象值以及版本号都必须满足期望值才能写入成功，这也就解决了反复读写时，无法预知值是否已被修改的窘境。AtomicStampedReference 底层是通过一个叫 Pair 的私有内部类来存储数据和版本号，并构造 valotile 修饰的私有实例。AtomicStampedReference 类的 compareAndSet() 方法在更新时对数据和版本号同时进行比较，只有两者都符合预期时才会调用 casPair() 方法，而 casPair() 方法最终还是调用的 Unsafe 类的方法。compareAndSet() 源码如下：
```java
public boolean compareAndSet(V expectedReference, V newReference, int expectedStamp, int newStamp) {
        Pair<V> current = pair;
        return expectedReference == current.reference && expectedStamp == current.stamp &&
            ((newReference == current.reference && newStamp == current.stamp) || casPair(current, Pair.of(newReference, newStamp)));
    }
```
使用 AtomicStampedReference 类解决 ABA 问题：
```java
import java.util.concurrent.atomic.AtomicStampedReference;

public class CAS_ABA_A {

    private static AtomicStampedReference<Integer> atomicStampedRef =
        new AtomicStampedReference<>(1, 0);

    public static void main(String[] args) {
        Thread main = new Thread(() -> {
            System.out.println(Thread.currentThread().getName() + ", 初始值 a = " + atomicStampedRef.getReference());
            int stamp = atomicStampedRef.getStamp();        //获取当前标识别
            try {
                Thread.sleep(1000);     //等待1秒 ，以便让干扰线程执行
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
            /* //此时 expectedReference 未发生改变，但是 stamp 已经被修改了，所以CAS失败 */
            boolean isCASSuccess = atomicStampedRef.compareAndSet(1, 2, stamp, stamp + 1);
            System.out.println(Thread.currentThread().getName() + ", CAS操作结果: " + isCASSuccess);
        }, "主操作线程");

        Thread other = new Thread(() -> {
            try {       //确保让main线程优先执行
                Thread.sleep(500);
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
            atomicStampedRef.compareAndSet(1, 2, atomicStampedRef.getStamp(), atomicStampedRef.getStamp() + 1);
            System.out.println(Thread.currentThread().getName() + ", [increment], a = " + atomicStampedRef.getReference());
            atomicStampedRef.compareAndSet(2, 1, atomicStampedRef.getStamp(), atomicStampedRef.getStamp() + 1);
            System.out.println(Thread.currentThread().getName() + ", [decrement], a = " + atomicStampedRef.getReference());
        }, "干扰线程");

        main.start();
        other.start();
    }
}
```

### AtomicMarkableReference
AtomicMarkableReference 的实现原理与 AtomicStampedReference 类似，只不过 AtomicMarkableReference 与 AtomicStampedReference 不同的是，AtomicMarkableReference 维护的是一个 boolean 类型的标识，也就是说标识会在 true 和 false 两种状态之间切换。经过测试，这种方式并不能完全防止 ABA 问题的发生，只能减少 ABA 问题发生的概率。所以，如果要完全杜绝 ABA 问题的发生，应该使用 AtomicStampedReference 原子类更新对象，而对于 AtomicMarkableReference 来说只能减少 ABA 问题的发生概率，并不能杜绝。

> 参考：
https://www.jianshu.com/p/8b227a8adbc1
https://blog.csdn.net/mmoren/article/details/79185862