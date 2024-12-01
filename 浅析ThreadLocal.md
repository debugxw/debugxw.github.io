---
title: 浅析ThreadLocal
excerpt: 在多线程程序中为每个线程提供独立的变量副本，保证线程安全性
date: 2019-07-16 09:02:18
tags: 并发
image: /img/theme/006.jpg
---

ThreadLocal 的作用是提供线程内的局部变量，说白了，就是在各线程内部创建一个变量的副本，相比于使用各种锁机制访问变量，ThreadLocal 的思想就是用空间换时间，使各线程都能访问属于自己这一份的变量副本，线程之间的变量值互不干扰，减少同一个线程内的多个函数或者组件之间一些公共变量传递的复杂度

### 一、使用示例
```java
public class Main {

    private static final ThreadLocal<Integer> threadLocal =
        new ThreadLocal<Integer>() {
            @Override
            protected Integer initialValue() {
                return Integer.valueOf(0);
            }
        };

    public static void main(String[] args) {
        Thread[] threads = new Thread[5];
        for (int i = 0; i < 5; i++) {
            threads[i] = new Thread(() -> {
                System.out.println(Thread.currentThread() + "'s initial value: " + threadLocal.get());
                for (int j = 0; j < 10; j++) {
                    threadLocal.set(threadLocal.get() + j);
                }
                System.out.println(Thread.currentThread() + "'s last value: " + threadLocal.get());
            });
        }

        for (Thread t : threads)
            t.start();
    }
}

/*
    运行结果：
        Thread[Thread-0,5,main]'s initial value: 0
        Thread[Thread-1,5,main]'s initial value: 0
        Thread[Thread-1,5,main]'s last value: 45
        Thread[Thread-2,5,main]'s initial value: 0
        Thread[Thread-2,5,main]'s last value: 45
        Thread[Thread-0,5,main]'s last value: 45
        Thread[Thread-3,5,main]'s initial value: 0
        Thread[Thread-3,5,main]'s last value: 45
        Thread[Thread-4,5,main]'s initial value: 0
        Thread[Thread-4,5,main]'s last value: 45
*/
```
从运行结果可以看出，每个线程之间的值相互不影响，且初始值都为 0。

### 二、实现原理

![ThreadLocal原理图](../../../../img/jdk/ThreadLocal.png)

首先，在 Thread 类中定义了一个 threadLocals 字段，它是 ThreadLocal.ThreadLocalMap 对象的引用，默认值是 null。ThreadLocal.ThreadLocalMap 对象表示了一个以开放地址形式的散列表。当我们在某个线程中第一次调用 ThreadLocal 对象的 get / set 方法时，会为当前线程创建一个 ThreadLocalMap 对象。也就是**每个线程都各自有一张独立的散列表，以 ThreadLocal 对象作为散列表的 key，set 方法中的值作为 value**。也就是说，相同的 key 在不同的散列表中的值必然是独立的，每个线程都是在各自的散列表中执行操作。

### 二、ThreadLocalMap.Entry
ThreadLocalMap 中用内部静态类 Entry 表示了散列表中的每一个条目，源码如下：
```java
static class Entry extends WeakReference<ThreadLocal<?>> {
    /** The value associated with this ThreadLocal. */
    Object value;

    Entry(ThreadLocal<?> k, Object v) {
        super(k);
        value = v;
    }
}
```
可以看出 Entry 类继承了 WeakRefrence 类，所以一个条目就是一个弱引用类型的对象（要搞清楚，持有 weakRefrence 对象的引用是个强引用），那么这个 weakRefrence 对象保存了谁的弱引用呢？我们看到构造函数中有个 supe(k)，k 是 ThreadLocal 类型对象，super 表示是调用父类的构造函数，所以说**一个 entry 对象中存储了 ThreadLocal 对象的弱引用和这个 ThreadLocal 对应的 value 对象的强引用**。[有关Java中的四种引用参考](https://debugxw.github.io/2019/07/14/Java%E4%B8%AD%E7%9A%84%E5%9B%9B%E7%A7%8D%E5%BC%95%E7%94%A8/)

### 三、ThreadLocal内存泄漏问题
从上面的分析中可以看出，ThreadLocal 的实现原理是每一个 Thread 维护一个 ThreadLocalMap 映射表，映射表的 key 是 ThreadLocal 实例，并且使用的是 ThreadLocal 的弱引用，value 是具体需要存储的 Object。下面用一张图展示这些对象之间的引用关系，实线箭头表示强引用，虚线箭头表示弱引用：

![](../../../../img/jdk/ThreadLocalMemoryLeak.png)

从上图可以看出，如果 ThreadLocal 没有外部强引用，当发生垃圾回收时，这个 ThreadLocal 一定会被回收（弱引用的特点是不管当前内存空间足够与否，其所引用对象在 GC 时都会被回收），这样就会导致 ThreadLocalMap 中出现 key 为 null 的 Entry，外部将不能获取这些 key 为 null 的 Entry 的 value，并且如果当前线程一直存活，那么就会存在一条强引用链：Thread Ref -> Thread -> ThreaLocalMap -> Entry -> value，导致 value 对应的 Object 一直无法被回收，产生内存泄露。

查看源码会发现，ThreadLocal 的 get、set 和 remove 方法都实现了对所有 key 为 null 的 value 的清除，但仍可能会发生内存泄露，因为可能使用了 ThreadLocal 的 get 或 set 方法后发生 GC，此后不调用 get、set 或 remove 方法，则 key 为 null 的 value 就不会被清除。

解决办法是每次使用完 ThreadLocal 都调用它的 remove() 方法清除数据，或者按照 JDK 的建议将 ThreadLocal 变量定义成 private static，这样就一直存在 ThreadLocal 的强引用，也就能保证任何时候都能通过 ThreadLocal 的弱引用访问到 Entry 的 value 值，进而清除掉。

### 四、源码分析
##### 1、set()
```java
public void set(T value) {
    Thread t = Thread.currentThread();  // 获取当前线程
    ThreadLocalMap map = getMap(t); // 获得当前线程的 ThreadLocalMap
    if (map != null)    // 如果存在则调用 ThreadLocalMap.set() 设置 value
        map.set(this, value);
    else    // 如果不存在，则先创建 map，再设置 value
        createMap(t, value);
}

ThreadLocalMap getMap(Thread t) {
    return t.threadLocals;
}
```
set() 方法会先获取当前线程实例和当前线程的 threadLocals，如果 threadLocals 不为空，则调用 ThreadLocalMap.set() 方法并设置 key-value。如果 threadLocals 为空，则为当前线程创建 threadLocals 并设置 value。

##### 2、createMap()
createMap() 用于创建线程的 ThreadLocalMap 对象并设置初始值：
```java
void createMap(Thread t, T firstValue) {
    t.threadLocals = new ThreadLocalMap(this, firstValue);
}

ThreadLocalMap(ThreadLocal<?> firstKey, Object firstValue) {
    table = new Entry[INITIAL_CAPACITY];    // 初始化 table，并将其大小设置为 INITIAL_CAPACITY = 16
    int i = firstKey.threadLocalHashCode & (INITIAL_CAPACITY - 1);  // 计算索引
    table[i] = new Entry(firstKey, firstValue); // 设置值
    size = 1;
    setThreshold(INITIAL_CAPACITY); // 设置阈值，当 table 的 size 大于或等于阈值时，table 会进行扩容
}
```

##### 3、ThreadLocalMap.set()
```java
private void set(ThreadLocal<?> key, Object value) {
    // 通过 key 的 hash 值找到 key 在数组中的下标
    Entry[] tab = table;
    int len = tab.length;
    int i = key.threadLocalHashCode & (len-1);

    /**
     * 根据获取到的索引进行循环，如果当前索引上的 table[i] 不为空，在没有 return 的情况下，
     * 就使用 nextIndex() 获取下一个 hash 位置
     */
    for (Entry e = tab[i];
        e != null;
        e = tab[i = nextIndex(i, len)]) {
        ThreadLocal<?> k = e.get();

        // 如果 key 相等表示之前已经存放了相同 key 的值，此时只需要更新 value 即可
        if (k == key) {
            e.value = value;
            return;
        }

        // 由于 key 为弱引用，所以它是会被回收的。如果它被回收了，则说明该 entry 已经是个过期的数据
        // 这个时候说明改 table[i] 可以重新使用，用新的 key-value 将其替换，并删除其他无效的 entry
        if (k == null) {
            replaceStaleEntry(key, value, i);
            return;
        }
    }
    
    // 执行到这一步，说明没有发现 key 相同的 entry，也没有发现可以存放数据的过期 entry。
    // 但是发现了空插槽，则需要新建 entry 存放数据，并将其放入空插槽
    tab[i] = new Entry(key, value);
    int sz = ++size;
    /**
     * cleanSomeSlots 用于清除那些 e.get()==null，也就是table[index] != null && table[index].get()==null
     * 之前提到过，这种数据 key 关联的对象已经被回收，所以这个 Entry(table[index]) 可以被置 null。
     * 如果没有清除任何 entry，并且当前使用量达到了负载因子所定义(长度的 2/3)，那么进行 rehash()
     */
    if (!cleanSomeSlots(i, sz) && sz >= threshold)
        rehash();
}
```
1. 首先还是根据 key 计算出位置 i，然后查找 i 位置上的 Entry。
2. 若是 Entry 已经存在并且 key 等于传入的 key，那么这时候直接给这个 Entry 赋新的 value 值。
3. 若是 Entry 存在，但是 key 为 null，则调用 replaceStaleEntry 来替换这个 key 为空的 Entry。
4. 不断循环检测，直到 e == null，这时候要是还没在循环过程中 return，那么就在这个 null 的位置新建一个 Entry，并且插入，同时 size 增加 1。
5. 最后调用 cleanSomeSlots 进行相关的清理工作

**解决 hash 冲突的方法**
如果在计算某个 Entry 的下标时发生了 hash 冲突，从 nextIndex() 方法中可以看出，ThreadLocal 会采用线性探测的方法来解决：
```java
private static int nextIndex(int i, int len) {
    return ((i + 1 < len) ? i + 1 : 0);
}
```

##### 4、ThreadLocalMap.replaceStaleEntry()
```java
// 替换无效 Entry
private void replaceStaleEntry(ThreadLocal<?> key, Object value,
                                       int staleSlot) {
    Entry[] tab = table;
    int len = tab.length;
    Entry e;

    /**
     * 根据传入的无效 Entry 的位置（staleSlot），向前扫描
     * 一段连续的 Entry（这里的连续是指一段相邻的 Entry 并且 table[i] != null），
     * 直到找到一个无效 Entry，或者扫描完也没找到
     */
    int slotToExpunge = staleSlot;      // 之后用于清理的起点
    for (int i = prevIndex(staleSlot, len);
         (e = tab[i]) != null; i = prevIndex(i, len))
        if (e.get() == null)
            slotToExpunge = i;

    // 向后扫描一段 Entry
    for (int i = nextIndex(staleSlot, len);
                 (e = tab[i]) != null; i = nextIndex(i, len)) {
        ThreadLocal<?> k = e.get();

        // 如果找到了 key，将其与传入的无效 Entry 替换，也就是与 table[staleSlot] 进行替换
        if (k == key) {
            e.value = value;

            tab[i] = tab[staleSlot];
            tab[staleSlot] = e;

            // 如果向前查找没有找到无效 Entry，则更新 slotToExpunge 为当前值 i
            if (slotToExpunge == staleSlot)
                slotToExpunge = i;
            cleanSomeSlots(expungeStaleEntry(slotToExpunge), len);
            return;
        }

        // 如果向前查找没有找到无效 Entry，并且当前向后扫描的 Entry无效，则更新 slotToExpunge 为当前值 i
        if (k == null && slotToExpunge == staleSlot)
            slotToExpunge = i;
    }

    /**
     * 如果没有找到 key，也就是说 key 之前不存在 table 中
     * 就直接最开始的无效 Entry——tab[staleSlot] 上直接新增即可
     */
    tab[staleSlot].value = null;
    tab[staleSlot] = new Entry(key, value);

    /**
     * slotToExpunge != staleSlot，说明存在其他的无效 Entry 需要进行清理。
     */
    if (slotToExpunge != staleSlot)
        cleanSomeSlots(expungeStaleEntry(slotToExpunge), len);
}
```
简单来说，ThreadLocalMap.set() 调用 replaceStaleEntry() 是因为在设置 value 时出现了某个 Entry.key 为 null 的情况，那么在设置新值的同时会从该位置向前向后寻找一段连续（Entry 不为空）的 Entry，并检查这段 Entry 是否存在 Entry.key 为 null 的情况，如存在，则进行清理工作。

##### 5、ThreadLocalMap.expungeStaleEntry()
```java
/**
 * 连续段清除
 * 根据传入的 staleSlot，清理对应的无效 entry——table[staleSlot]，
 * 并且根据当前传入的 staleSlot，向后扫描一段连续的 Entry（这里的连续是指一段相邻的 Entry 并且 table[i] != null），
 * 对可能存在 hash 冲突的 Entry 进行 rehash，并且清理遇到的无效 Entry。
 *
 * @param staleSlot key 为 null，需要无效 Entry 所在的 table 中的索引
 * @return 返回下一个为空的 solt 的索引。
 */
private int expungeStaleEntry(int staleSlot) {
    Entry[] tab = table;
    int len = tab.length;

    // 清理无效 Entry
    tab[staleSlot].value = null;
    tab[staleSlot] = null;
    size--;

    Entry e;
    int i;
    // 从 staleSlot 开始向后扫描一段连续的 Entry
    for (i = nextIndex(staleSlot, len);
         (e = tab[i]) != null; i = nextIndex(i, len)) {
        ThreadLocal<?> k = e.get();
        // 如果遇到 key 为 null，表示无效 Entry，进行清理。 
        if (k == null) {
            e.value = null;
            tab[i] = null;
            size--; 
        } else {
            int h = k.threadLocalHashCode & (len - 1);
            // 计算出来的索引 h，与其现在所在位置的索引 i 不一致，置空当前的 table[i]
            // 从 h 开始向后线性探测到第一个空的 slot，把当前的 Entry挪过去。
            if (h != i) {
                tab[i] = null;

                while (tab[h] != null)
                    h = nextIndex(h, len);
                tab[h] = e;
            }
        }
    }
    // 下一个为空的 solt
    return i;
}
```

简单来说，expungeStaleEntry() 会根据 staleSlot 向后清除一个连续段中的无效 Entry，并在清理的过程中对 key 不为空的 Entry 进行 rehash。为什么要进行 rehash 呢？因为我们在清理的过程中会把某个 Entry 设为 null，如果这个 Entry 和它前后两个 Entry 都发生了哈希冲突，那么下一次在查找这个 Entry 后面这个 Entry 的时候会因为遇到 null 而查找失败，例如：

<center><img src="../../../../img/jdk/ThreadLocal_rehash.png" width="100%" height="100%" /></center>

假设这里的 hash(e1) = hash(e2) = hash(e3)，那么在插入 e2 和 e3 时会发生哈希冲突。而我们知道 ThreadLocal 解决哈希冲突的方法是线性探测，所以 e2 和 e3 会依次放在 e1 后面。如果这时 e2 失效，即 e2 = null，在查找 e3 时，首先通过 e3 的哈希值计算数组下标，会发现 e3 对应的下标应该在 e1 的位置，而 e1 != e3，这时会按照线性探测的方式继续向后查找。而 e1 的下一个位置为 null，这时就会认为 e3 不存在，查找失败。

##### 6、ThreadLocalMap.cleanSomeSlots()
```java
/**
 * 启发式的扫描清除，扫描次数由传入的参数 n 决定
 *
 * @param i 从 i 向后开始扫描（不包括 i，因为索引为 i 的 Slot 肯定为 null）
 *
 * @param n 控制扫描次数，正常情况下为 log2(n) ，
 * 如果找到了无效 Entry，会将 n 重置为 table 的长度 len，进行段清除。
 *
 * map.set() 点用的时候传入的是元素个数，replaceStaleEntry() 调用的时候传入的是 table 的长度 len
 *
 * @return true if any stale entries have been removed.
 */
private boolean cleanSomeSlots(int i, int n) {
    boolean removed = false;
    Entry[] tab = table;
    int len = tab.length;
    do {
        i = nextIndex(i, len);
        Entry e = tab[i];
        if (e != null && e.get() == null) {
            n = len;    // 重置 n 为 len
            removed = true;
            // 依然调用 expungeStaleEntry 来进行无效 Entry 的清除
            i = expungeStaleEntry(i);
        }
    } while ( (n >>>= 1) != 0); // 无符号的右移动，可以用于控制扫描次数在 log2(n)
    return removed;
}
```
该方法会从位置 i 开始向后扫描，清除无效的 Entry，而循环次数为什么是 log2(n) 次之内呢？我也不知道！[听说](https://www.jianshu.com/p/56f64e3c1b6c)，它执行对数数量的扫描，是一种**基于不扫描（快速但保留垃圾）和扫描所有元素之间的平衡**。

##### 7、get()
```java
public T get() {
    Thread t = Thread.currentThread();
    ThreadLocalMap map = getMap(t); // 获取当前线程的 ThreadLocalMap 对象
    if (map != null) {
        ThreadLocalMap.Entry e = map.getEntry(this);    // 找到相应的 Entry
        if (e != null) {
            @SuppressWarnings("unchecked")
            T result = (T)e.value;  // 如果 e 不为 null，直接返回 e.value
            return result;
        }
    }
    // 如果当前线程的 ThreadLocalMap 不存在则进行初始化
    return setInitialValue();
}
```
get() 方法用于获取线程在当前 ThreadLocal 对象下所对应的实例，和 set() 方法类似，先获取线程对象的 ThreadLocalMap 实例，如果该实例为空，则用 setInitialValue() 进行初始化。如果不为空，则根据 ThreadLocal 获得 map 中相应的 Entry。如果 Entry 不为空，则直接返回 Entry.value，否则用 setInitialValue() 进行初始化。

##### 8、ThreadLocalMap.getEntry()
getEntry() 方法会根据 ThreadLocal 在当前线程中的 Entry：
```java
private Entry getEntry(ThreadLocal<?> key) {
    int i = key.threadLocalHashCode & (table.length - 1);
    Entry e = table[i];
    // 如果 e 不为空且 e 未失效，直接返回
    if (e != null && e.get() == key)
        return e;
    else
        return getEntryAfterMiss(key, i, e);
}
```

##### 9、ThreadLocalMap.getEntryAfterMiss()
```java
private Entry getEntryAfterMiss(ThreadLocal<?> key, int i, Entry e) {
    Entry[] tab = table;
    int len = tab.length;

    while (e != null) {
        ThreadLocal<?> k = e.get();
        if (k == key)   // 说明找到了相应 Entry，直接返回
            return e;
        if (k == null)  // 说明这是一个无效 Entry，需要清理
            expungeStaleEntry(i);
        else
            i = nextIndex(i, len);
        e = tab[i];
    }
    return null;
}
```
这个方法我们还得结合上一步看，上一步是因为不满足 e != null && e.get() == key 才沦落到调用 getEntryAfterMiss 的，所以首先 e 如果为 null 的话，那么 getEntryAfterMiss 还是直接返回 null。如果是不满足 e.get() == key，那么进入 while 循环，如果在循环的过程中找到一个 Entry 满足 k == key，则直接返回。如果找到一个 Entry 且 Entry.key == null，则调用 expungeStaleEntry() 清理 Entry。

##### 10、setInitialValue()
该方法和 set() 方法相似，只不过 set() 会设置给定的值，而 setInitialValue() 会设置 initialValue() 返回的初始值。而在 ThreadLocal 中，initialValue() 总是返回 null，所以我们可以继承 ThreadLocal 并重写 initialValue()，这样就可以设置相应的初始值，定制自己的 ThreadLocal。
```java
private T setInitialValue() {
    T value = initialValue();   // 获得初始值
    Thread t = Thread.currentThread();
    ThreadLocalMap map = getMap(t);
    if (map != null)
        map.set(this, value);
    else
        createMap(t, value);
    return value;
}

protected T initialValue() {
    return null;
}
```

> 参考：
  [ThreadLocal源码深度剖析](https://juejin.im/post/5a5efb1b518825732b19dca4)
  [ThreadLocal原理及使用示例](https://www.cnblogs.com/nullzx/p/7553538.html)
  [ThreadLocal内存泄漏问题精简说](https://cloud.tencent.com/developer/article/1086165)
  [ThreadLocal源码解析](https://brightloong.github.io/2018/05/28/ThreadLocal%E6%BA%90%E7%A0%81%E8%A7%A3%E6%9E%90/)
