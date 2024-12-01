---
title: Java中的四种引用
excerpt: 图书馆的书签、银行账户的卡号、快递的寄送地址
date: 2019-07-14 20:06:29
tags: Java
image: /img/theme/009.jpg
---

在 JDK1.2 之前，java 中的引用定义很传统：如果 reference 类型的数据中存储的数值代表的是另外一块内存的起始地址，就称这块内存代表着引用。这种定义很纯粹，但对于描述一些 "弃之可惜，食之无味" 的对象就显得无能为力。我们希望描述这样一类对象：当内存空间足够时，则能保留在内存之中；如果内存空间在进行完垃圾回收之后还是很紧张，则可以抛弃这些对象。于是在 JDK1.2 之后，java 对引用的概念进行了扩充，将引用分为强引用 (Strong Reference)、软引用 (Soft Reference)、弱引用 (Weak Reference)、虚引用 (Phantom Reference) 4 种，这 4 种引用强度依次逐渐减弱。

### 一、强引用
强引用就是在程序代码中普遍存在的，类似 Object obj = new Object() 这类的引用，只要强引用还存在，垃圾回收器就永远不会回收掉被引用的对象。
```java
public class StrongReferenceTest {

    public static int _1M = 1024 * 1024;

    public static void printlnMemory(String tag) {
        Runtime runtime = Runtime.getRuntime();
        System.out.println("\n" + tag + ":");
        System.out.println(runtime.freeMemory() / _1M + "M(free)/" + runtime.totalMemory() / _1M + "M(total)");
    }

    public static void main(String[] args) {
        printlnMemory("1、原可用内存和总内存");

        //实例化10M的数组并与strongReference建立强引用
        byte[] strongReference = new byte[10 * _1M];
        printlnMemory("2、实例化10M的数组，并建立强引用");
        System.out.println("strongReference : " + strongReference);

        System.gc();
        printlnMemory("3、GC后");
        System.out.println("strongReference : " + strongReference);

        //strongReference = null 后，强引用就断开了
        strongReference = null;
        printlnMemory("4、强引用断开后");
        System.out.println("strongReference : " + strongReference);

        System.gc();
        printlnMemory("5、GC后");
        System.out.println("strongReference : " + strongReference);
    }
}

/*
    运行结果：

    1、原可用内存和总内存:
    17M(free)/19M(total)

    2、实例化10M的数组，并建立强引用:
    7M(free)/19M(total)
    strongReference : [B@7ea987ac

    3、GC后:
    8M(free)/19M(total)
    strongReference : [B@7ea987ac

    4、强引用断开后:
    8M(free)/19M(total)
    strongReference : null

    5、GC后:
    18M(free)/19M(total)
    strongReference : null
*/
```

### 二、软引用
软引用用来描述一些还有用但并非必须的对象。对于软引用关联的对象，在系统将要发生内存溢出异常之前，将会把这些对象，列进回收范围之中进行第二次回收，如果这次回收还没有足够的内存，才会抛出内存溢出异常。
```java
import java.lang.ref.SoftReference;

public class SoftReferenceTest {

    public static int _1M = 1024 * 1024;

    public static void printlnMemory(String tag) {
        Runtime runtime = Runtime.getRuntime();
        System.out.println("\n" + tag + ":");
        System.out.println(runtime.freeMemory() / _1M + "M(free)/" + runtime.totalMemory() / _1M + "M(total)");
    }

    public static void main(String[] args) {
        SoftReferenceTest.printlnMemory("1、原可用内存和总内存");

        // 建立软引用
        SoftReference<byte[]> softRerference = new SoftReference<>(new byte[10 * _1M]);
        printlnMemory("2、实例化10M的数组，并建立软引用");
        System.out.println("softRerference.get() : " + softRerference.get());

        System.gc();
        printlnMemory("3、内存可用容量充足，GC后");
        System.out.println("softRerference.get() : " + softRerference.get());

        // 实例化一个10M的数组，使内存不够用，并建立软引用
        // 内存可用量不足时，GC后byte[10*_1M]被回收
        SoftReference<byte[]> softRerference2 = new SoftReference<>(new byte[10 * _1M]);
        printlnMemory("4、再次实例化一个10M的数组后");
        System.out.println("softRerference.get() : " + softRerference.get());
        System.out.println("softRerference2.get() : " + softRerference2.get());
    }
}

/*
    运行结果：

    1、原可用内存和总内存:
    17M(free)/19M(total)

    2、实例化10M的数组，并建立软引用:
    7M(free)/19M(total)
    softRerference.get() : [B@7ea987ac

    3、内存可用容量充足，GC后:
    8M(free)/19M(total)
    softRerference.get() : [B@7ea987ac

    4、再次实例化一个10M的数组后:
    8M(free)/19M(total)
    softRerference.get() : null
    softRerference2.get() : [B@12a3a380
*/
```

### 三、弱引用
弱引用也是用来描述非必须对象的，但是它的强度比软引用更弱一些，被弱引用关联的对象只能生存到下一次垃圾回收发生之前。当垃圾收集器工作时，无论当前内存是否足够，都会回收掉只被弱引用关联的对象。
```java
import java.lang.ref.WeakReference;

public class WeakReferenceTest {

    public static int _1M = 1024 * 1024;

    public static void printlnMemory(String tag) {
        Runtime runtime = Runtime.getRuntime();
        System.out.println("\n" + tag + ":");
        System.out.println(runtime.freeMemory() / _1M + "M(free)/" + runtime.totalMemory() / _1M + "M(total)");
    }

    public static void main(String[] args) {
        printlnMemory("1、原可用内存和总内存");

        //创建弱引用
        WeakReference<byte[]> weakRerference = new WeakReference<>(new byte[10 * _1M]);
        printlnMemory("2、实例化10M的数组，并建立弱引用");
        System.out.println("weakRerference.get() : " + weakRerference.get());

        System.gc();
        printlnMemory("3、GC后");
        System.out.println("weakRerference.get() : " + weakRerference.get());
    }
}

/*
    运行结果：
    
    1、原可用内存和总内存:
    17M(free)/19M(total)
    
    2、实例化10M的数组，并建立弱引用:
    7M(free)/19M(total)
    weakRerference.get() : [B@7ea987ac
    
    3、GC后:
    18M(free)/19M(total)
    weakRerference.get() : null
*/
```

### 四、虚引用
也成为幽灵引用或者幻影引用，它是最弱的一种引用关系。一个对象是否有虚引用的存在，完全不会对其生存时间构成影响，也无法通过虚引用来取得一个对象的实例。为一个对象设置虚引用的唯一目的就是在这个对象被垃圾回收器回收时收到一个系统通知。（虚引用一般会配合引用队列 (ReferenceQueue) 来使用。当某个被虚引用指向的对象被回收时，我们可以在其引用队列中得到这个虚引用的对象作为其所指向的对象被回收的一个通知。）

上面这一段话摘自《深入理解 Java 虚拟机：JVM 高级特性与最佳实战》。按理说如果一个对象只存在虚引用，那么它会被 GC 回收，但是在实际的测试过程中却不是这样的：
```java
import java.lang.ref.PhantomReference;
import java.lang.ref.ReferenceQueue;

public class PhantomReferenceTest {

    public static int _1M = 1024 * 1024;

    public static void printlnMemory(String tag) {
        Runtime runtime = Runtime.getRuntime();
        System.out.println("\n" + tag + ":");
        System.out.println(runtime.freeMemory() / _1M + "M(free)/" + runtime.totalMemory() / _1M + "M(total)");
    }

    public static void main(String[] args) throws InterruptedException {

        PhantomReferenceTest.printlnMemory("1、原可用内存和总内存");
        byte[] bytes = new byte[10 * _1M];
        printlnMemory("2、实例化10M的数组后");

        // 建立引用队列和虚引用
        ReferenceQueue<byte[]> referenceQueue = new ReferenceQueue<>();
        PhantomReference<byte[]> phantomReference = new PhantomReference<>(bytes, referenceQueue);

        printlnMemory("3、建立虚引用后");
        System.out.println("phantomReference : " + phantomReference);
        System.out.println("phantomReference.get() : " + phantomReference.get());
        System.out.println("referenceQueue.poll() : " + referenceQueue.poll());

        // 断开byte[10*_1M]的强引用
        bytes = null;
        printlnMemory("4、执行bytes=null;强引用断开后");

        System.gc();
        PhantomReferenceTest.printlnMemory("5、GC后");
        System.out.println("phantomReference : " + phantomReference);
        System.out.println("phantomReference.get() : " + phantomReference.get());
        System.out.println("referenceQueue.poll() : " + referenceQueue.poll());

        // 断开虚引用
        phantomReference = null;
        System.gc();
        printlnMemory("6、断开虚引用后 GC");
        System.out.println("phantomReference : " + phantomReference);
        System.out.println("referenceQueue.poll() : " + referenceQueue.poll());
    }
}

/*
    运行结果：
    
    1、原可用内存和总内存:
    17M(free)/19M(total)
    
    2、实例化10M的数组后:
    7M(free)/19M(total)
    
    3、建立虚引用后:
    7M(free)/19M(total)
    phantomReference : java.lang.ref.PhantomReference@7ea987ac
    phantomReference.get() : null
    referenceQueue.poll() : null
    
    4、执行bytes=null;强引用断开后:
    7M(free)/19M(total)
    
    5、GC后:
    8M(free)/19M(total)
    phantomReference : java.lang.ref.PhantomReference@7ea987ac
    phantomReference.get() : null
    referenceQueue.poll() : java.lang.ref.PhantomReference@7ea987ac
    
    6、断开虚引用后 GC:
    18M(free)/19M(total)
    phantomReference : null
    referenceQueue.poll() : null
*/
```
从运行结果中可以看出，当使用 bytes = null 断开强引用之后执行 System.gc()，对象并没有被回收，而在使用 phantomReference = null 断开虚引用之后，对象才被回收。这是怎么回事呢？

### 五、Reference 类
Reference 是 SoftReference、WeakReference 和 PhantomReference 三者的父类。在 Reference 类中有一个私有的 referent 字段，当我们使用构造函数新建一个 Reference 对象时，会传递一个实例对象的引用赋值给 referent。当使用 Reference.get() 方法获取对象时，实际上是返回了 referent。源码如下：
```java
    ...
    private T referent;         /* Treated specially by GC */
    
    Reference(T referent, ReferenceQueue<? super T> queue) {
        this.referent = referent;
        this.queue = (queue == null) ? ReferenceQueue.NULL : queue;
    }
    
    public T get() {
        return this.referent;
    }
    ...
```
**本质上 Reference.referent 也是一个强引用，只不过是 GC 在执行垃圾回收时，如果发现某个对象只被 SoftReference 或 WeakReference 引用，那么 JVM 都是先将其 referenct 字段设置为 null，之后将软引用或弱引用，加入到关联的引用队列中。我们可以认为 JVM 先回收堆对象占用的内存，然后才将软引用或弱引用加入到引用队列。**

<font color="cc0000">**而虚引用则不同，JVM 不会自动将虚引用的 referent 字段设置成 null，而是先保留堆对象的内存空间，直接将 PhantomReference 加入到关联的引用队列，也就是说如果我们不手动调用 PhantomReference.clear() 或者直接将 PhantomReference 对象设为 null，虚引用指向的堆对象内存是不会被释放的。**</font>

referent 是 java.lang.ref.Reference 类的私有字段，虽然没有暴露出共有 API 来访问这个字段，但是我们可以通过反射拿到这个字段的值，这样就能知道引用被加入到引用队列的时候，referent 到底是不是 null。SoftReference 和WeakReference 是一样的，这里我们以 WeakReference 为例：
```java
public class TestWeakReference {

    private static volatile boolean isRun = true;
    private static volatile ReferenceQueue<String> referenceQueue = new ReferenceQueue<>();

    public static void main(String[] args) throws Exception {
        String abc = new String("abc");
        System.out.println(abc.getClass() + "@" + abc.hashCode());

        new Thread(() -> {
            while (isRun) {
                Object o = referenceQueue.poll();
                if (o != null) {
                    try {
                        Field referent = Reference.class
                            .getDeclaredField("referent");
                        referent.setAccessible(true);
                        Object result = referent.get(o);
                        System.out.println("gc will collect:"
                            + result.getClass() + "@"
                            + result.hashCode());
                    } catch (Exception e) {
                        e.printStackTrace();
                    }
                }
            }
        }).start();

        // 对象是弱可达的
        WeakReference<String> weak = new WeakReference<>(abc,
            referenceQueue);
        System.out.println("weak = " + weak);

        // 清除强引用,触发GC
        abc = null;
        System.gc();
        Thread.sleep(1000);
        isRun = false;
    }
}
```
运行这段代码会发现，我们创建的 Thread 中报空指针异常。当我们清除强引用，触发 GC 的时候，JVM 检测到 new String("abc") 这个堆中的对象只有 WeakReference，那么 JVM 会释放堆对象的内存，并自动将 WeakReference 的 referent 字段设置成 null，所以 result.getClass() 会报空指针异常。

与上面代码类似，我们将 WeakReference 替换成 PhantomReference：
```java
public class TestPhantomReference {

    private static volatile boolean isRun = true;
    private static volatile ReferenceQueue<String> referenceQueue = new ReferenceQueue<>();

    public static void main(String[] args) throws Exception {
        String abc = new String("abc");
        System.out.println(abc.getClass() + "@" + abc.hashCode());

        new Thread(() -> {
            while (isRun) {
                Object o = referenceQueue.poll();
                if (o != null) {
                    try {
                        Field rereferent = Reference.class
                            .getDeclaredField("referent");
                        rereferent.setAccessible(true);
                        Object result = rereferent.get(o);
                        System.out.println("gc will collect:"
                            + result.getClass() + "@"
                            + result.hashCode());
                    } catch (Exception e) {
                        e.printStackTrace();
                    }
                }
            }
        }).start();

        // 测试情况1:对象是虚可达的
        PhantomReference<String> phantom = new PhantomReference<>(abc,
            referenceQueue);
        System.out.println("phantom = " + phantom);

        // 测试情况2:对象是不可达的,直接就被回收了,不会加入到引用队列
        // new PhantomReference<>(abc, referenceQueue);

        // 清除强引用,触发GC
        abc = null;
        System.gc();
        Thread.sleep(1000);
        isRun = false;
    }
}

/*
    运行结果：
    
    class java.lang.String@96354
    phantom = java.lang.ref.PhantomReference@568db2f2
    gc will collect:class java.lang.String@96354
*/
```
很明显，当 PhantomReference 加入到引用队列的时候，referent 字段的值并不是 null，而且堆对象占用的内存空间仍然存在。也就是说对于虚引用，JVM 是先将其加入引用队列。由此可见，使用虚引用有潜在的内存泄露风险，因为 JVM 不会自动帮助我们释放，我们必须要保证它指向的堆对象是不可达的。从这点来看，虚引用其实就是强引用，当内存不足的时候，JVM 不会自动释放堆对象占用的内存。

**小结**
虚引用主要用来跟踪对象被垃圾回收器回收的活动。虚引用与软引用和弱引用的一个区别在于：虚引用必须和引用队列 (ReferenceQueue) 联合使用。当垃圾回收器回收一个对象时，如果发现它还有虚引用，就会把这个虚引用加入到与之关联的引用队列中。程序可以通过判断引用队列中是否已经加入了虚引用，来了解被引用的对象是否将要被回收。如果程序发现某个虚引用已经被加入到引用队列，那么就可以在所引用的对象的内存被回收之前采取必要的行动。


> 参考：
  [详解 Java 中的四种引用](https://blog.csdn.net/hacker_zhidian/article/details/83043270)
  [Java中的四种引用类型](https://juejin.im/post/5a5129f5f265da3e317dfc08)
  [java中虚引用PhantomReference与弱引用WeakReference（软引用SoftReference）的差别](https://blog.csdn.net/aitangyong/article/details/41358427)