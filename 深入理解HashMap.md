---
title: 深入理解HashMap（JDK1.8）
excerpt: 基于哈希表实现的键值对集合，具有快速的插入、查找和删除操作
date: 2019-03-16 12:20:45
tags: Java
image: /img/theme/018.jpg
---

HashMap 是 java 中用来存储 key-value 键值对的一种容器，其中的 key 和 value 都允许为 null。其底层的数据结构为数组 + 链表 + 红黑树，当链表长度达到 TREEIFY_THRESHOLD = 8 时，该链表会自动转化为红黑树，以提升 HashMap 的查询、插入效率，它实现了 Map<K, V>，Cloneable，Serializable接口

### 一、初始化
**三个最主要的构造函数**
```java
    public HashMap() {
        this.loadFactor = DEFAULT_LOAD_FACTOR; // all other fields defaulted
    }

    public HashMap(int initialCapacity) {
        this(initialCapacity, DEFAULT_LOAD_FACTOR);
    }
    
    public HashMap(int initialCapacity, float loadFactor) {
        if (initialCapacity < 0)
            throw new IllegalArgumentException("Illegal initial capacity: " +
                                               initialCapacity);
        if (initialCapacity > MAXIMUM_CAPACITY)
            initialCapacity = MAXIMUM_CAPACITY;
        if (loadFactor <= 0 || Float.isNaN(loadFactor))
            throw new IllegalArgumentException("Illegal load factor: " +
                                               loadFactor);
        this.loadFactor = loadFactor;
        this.threshold = tableSizeFor(initialCapacity);
    }
```
如果使用默认构造函数，则 HashMap 的初始化容量为 1 << 4 也就是 16，默认加载因子是 DEFAULT_LOAD_FACTOR = 0.75f，当 HashMap 底层的数组元素个数 > 数组容量 * 加载因子时，HashMap 将进行扩容操作，当然也可以在初始化时给定一个 loadFactor。如果在初始化时给定 initialCapacity，则初始化容量 C 需满足：C 是 2 的幂次方且 C >= initialCapacity 且 C <= (1 << 30)。其中的 tableSizeFor 方法保证函数返回值是大于等于给定参数 initialCapacity 最小的 2 的幂次方的数值，具体为：
```java
    static final int tableSizeFor(int cap) {
        int n = cap - 1;
        n |= n >>> 1;
        n |= n >>> 2;
        n |= n >>> 4;
        n |= n >>> 8;
        n |= n >>> 16;
        return (n < 0) ? 1 : (n >= MAXIMUM_CAPACITY) ? MAXIMUM_CAPACITY : n + 1;
    }
```
[参考](https://juejin.im/post/58f2f47061ff4b0058f4b7cc)

### 二、put() 操作
```java
    public V put(K key, V value) {
        return putVal(hash(key), key, value, false, true);
    }
```
```java
    static final int hash(Object key) {
        int h;
        return (key == null) ? 0 : (h = key.hashCode()) ^ (h >>> 16);
    }
```
```java
    final V putVal(int hash, K key, V value, boolean onlyIfAbsent,
                   boolean evict) {
        Node<K,V>[] tab; Node<K,V> p; int n, i;
        if ((tab = table) == null || (n = tab.length) == 0)
            n = (tab = resize()).length;
        if ((p = tab[i = (n - 1) & hash]) == null)
            tab[i] = newNode(hash, key, value, null);
        else {
            Node<K,V> e; K k;
            if (p.hash == hash &&
                ((k = p.key) == key || (key != null && key.equals(k))))
                e = p;
            else if (p instanceof TreeNode)
                e = ((TreeNode<K,V>)p).putTreeVal(this, tab, hash, key, value);
            else {
                for (int binCount = 0; ; ++binCount) {
                    if ((e = p.next) == null) {
                        p.next = newNode(hash, key, value, null);
                        if (binCount >= TREEIFY_THRESHOLD - 1) // -1 for 1st
                            treeifyBin(tab, hash);
                        break;
                    }
                    if (e.hash == hash &&
                        ((k = e.key) == key || (key != null && key.equals(k))))
                        break;
                    p = e;
                }
            }
            if (e != null) { // existing mapping for key
                V oldValue = e.value;
                if (!onlyIfAbsent || oldValue == null)
                    e.value = value;
                afterNodeAccess(e);
                return oldValue;
            }
        }
        ++modCount;
        if (++size > threshold)
            resize();
        afterNodeInsertion(evict);
        return null;
    }
```
向一个 HashMap 中添加一个元素，大致流程如下：
1. 对 key 求 hash 值，然后计算下标
  + hash = (key == null) ? 0 : (h = key.hashCode()) ^ (h >>> 16);
  + index = (capacity - 1) & hash;
在多数情况下，数组容量不会超过 2 ^ 16，所以如果直接用哈希值与数组容量进行与运算的话，哈希值的高十六位就用不到，所以进行 (h = key.hashCode()) ^ (h >>> 16) 操作，这样不仅利用上了 hash 值得高十六位，还减小了 hash 冲突的几率！
要计算一个 hash 的下标，通常是进行取余操作：hash % capacity。但因为 capacity 是 2 的幂次方，所以 hash % capacity = (capacity - 1) & hash，且位运算更加高效，所以采用位运算的方式计算下标，这也是为什么 HashMap 的容量要等于2的幂次方的原因。
2. 如果没有碰撞，直接放入数组
3. 如果发生碰撞，以链表的形式链接到后面（链接到链表最后，遍历的同时比较当前加入的节点是否已存在）
4. 如果链表的长度大于或等于阈值 TREEIFY_THRESHOLD，就把链表转换成红黑树
5. 如果节点已存在，就替换旧值
6. 如果桶（数组）满了（++size > threshold），就需要 resize

### 三、HashMap 的扩容操作
1. 容量扩充为原来的两倍，然后对每个节点重新计算哈希值
2. 这个值可能在两个地方，一个是原下标的位置，另一种是在下标为<原下标+原容量>的位置

假设 capacity = 10000，则 index = 1111 & hash。扩容为二倍后，capacity = 100000，则 index = 11111 & hash。扩容后 capacity - 1 的低四位没有变，而仅仅是多了一个最高位，而这个最高位（从右往左第五位）相对应的 hash 值的第五位只有 0 或 1 两种可能：如果为 0，则 index 不变；如果为 1，则 index = 原下标 + 原容量。

扩容时，会新建一个哈希数组，然后将原来数组中的每个元素移过来，这是一个非常耗时的操作。所以如果我们预先知道了有多少个键值对，那么在初始化时我们就可以给定一个容量，这样就可以减少 HashMap 扩容所带来的消耗。例如，如果我们知道键值对大概有 1000 个，那么就可以得到 1000 / 0.75 ≈ 1333，比 1333 大且为 2 的幂次方的最小数是 2048。但是如果我们将 HashMap 的初始化容量设置为 2048 就会可能会出现空间浪费的情况。因为当把一个键值对添加到 HashMap 时，可能有很多键值对都会发生哈希冲突，然后他们将会以链表或红黑树的方式连接到哈希数组中，所以哈希数组中不为空的元素不一定为 1000，可能为 200，也有可能是 300。所以在给定 HashMap 初始换容量时，不仅要考虑键值对的数量，还要考虑这些键值对发生哈希冲突的概率等等。

### 四、HashSet
HashSet 是 java 中用来存储**不能重复**且**无序**的数据的一种容器。但在本质上，HashSet 其实是用 HashMap 来存储数据的。在 HashSet 的源码中，有如下两个成员：
```java
    private transient HashMap<E,Object> map;

    // Dummy value to associate with an Object in the backing Map
    private static final Object PRESENT = new Object();
```
map 就是用来存储数据的容器，HashSet 将所有的数据存储在 map 的 key 中，因为 HashMap 中的 key 是唯一的，所以也就达到了 HashSet 存储不重复元素的目的。在向 HashSet 中添加一个元素时，实际上是调用了 HashMap 的 put() 方法：
```java
    public boolean add(E e) {
        return map.put(e, PRESENT)==null;
    }
```
可以看到，在添加元素时，将元素作为 key 添加到 map 中，而 value 则放入一个 Object 常量（本质上 value 没什么卵用），也就是说 map 中存储的所有键值对的 key 都不相同，而 value 都相同。
HashSet 中的其它方法，其实也都是直接调用了 HashMap 中的方法，例如：
```java
    public boolean remove(Object o) {
        return map.remove(o)==PRESENT;
    }
    
    public boolean contains(Object o) {
        return map.containsKey(o);
    }
    
    public int size() {
        return map.size();
    }
```