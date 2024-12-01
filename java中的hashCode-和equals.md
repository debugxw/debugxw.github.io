---
title: Java中的hashCode和equals方法
excerpt: 主要用于判断两个对象是否相同
date: 2019-04-20 10:53:00
tags: Java
toc: false
image: /img/theme/022.jpg
---

hashCode() 和 equals() 都是 Object 类中定义的方法，主要用来比较两个对象是否相同。hashCode() 方法在 Object 类中是一个本地方法，会将对象的内存地址转化为一个 int 数值并返回；而 equals() 方法仅仅比较的是两个对象的内存地址是否相同。他们在 Object 中的定义如下：
```java
public native int hashCode();

public boolean equals(Object obj) {
    return (this == obj);
}
```

如果一个类没有重写 equals() 方法，那么当使用 a.equals(b) 比较对象的时候相当于使用 == 运算符，比较的是对象的内存地址是否相同。如果要比较对象的内容，那么就需要重写 equals() 方法，像 java 中的 String 就重写了该方法，源码如下：
```java
public boolean equals(Object anObject) {
    if (this == anObject) {
        return true;
    }
    if (anObject instanceof String) {
        String anotherString = (String)anObject;
        int n = value.length;
        if (n == anotherString.value.length) {
            char v1[] = value;
            char v2[] = anotherString.value;
            int i = 0;
            while (n-- != 0) {
                if (v1[i] != v2[i])
                    return false;
                i++;
            }
            return true;
        }
    }
    return false;
}
```
但是在很多情况下（例如 java 容器中的 HashMap），比较一个对象是否相等都同时用到了 hashCode() 和 equals() 方法。使用的常规协定为：
+ hashCode() 为 true，equals() 不一定为 true
+ hashCode() 为 false，equals() 一定为 false
+ equals() 为 true，hashCode() 一定为 true
+ equals() 为 false，hashCode() 不一定为false

<center><img src="../../../../img/jdk/hashCode_equals.png" width="60%" height="60%" /></center>

之前在写一个编程题的时候需要判断相同的键值对 <K, V> 有多少个，于是就想到了用 HashMap<Node<Integer, Integer>, Integer> 来存储，Node<K, V> 是我自定义的一个类。考虑到 HashMap 在使用 put() 方法时使用到了 key.hashCode() 方法来确定元素在 HashMap 中的位置，于是重写了 Node 类的 hashCode() 方法，源码如下：
```java
import java.util.HashMap;

public class Main {

    static class Node<K, V> {
        K key;
        V value;

        Node(K key, V value) {
            this.key = key;
            this.value = value;
        }

        @Override
        public int hashCode() {
            return key.hashCode() * 13 + (value == null ? 0 : value.hashCode());
        }
    }

    public static void main(String[] args) {
        HashMap<Node<Integer, Integer>, Integer> hashMap = new HashMap<>();
        hashMap.put(new Node<>(3, 5), 1);

        System.out.println(hashMap.get(new Node<>(3, 5)));
        System.out.println(hashMap.containsKey(new Node<>(3, 5)));
    }
}
```
运行这个程序会发现程序的输出结果为 null 和 false，为什么会这样呢？虽然上面的三个 Node 对象在内存中的地址不同，但是他们的 hashCode() 返回的值是一样的，既然 hash 值是相同的，那为什么会出错呢？于是去查了一下 hashCode() 和 equals() 的用法，又看了一下 HashMap 的源码。发现原来在执行 put() 操作时，如果两个元素 hash 相同，那么这两个元素会放在同一个 hash 桶里，而如果要判断这两个元素是否相同，还要使用 equals() 方法，HashMap 中 putVal() 方法如下：
```java
final V putVal(int hash, K key, V value, boolean onlyIfAbsent, boolean evict) {
    Node<K,V>[] tab; Node<K,V> p; int n, i;
    if ((tab = table) == null || (n = tab.length) == 0)
        n = (tab = resize()).length;
    if ((p = tab[i = (n - 1) & hash]) == null)
        tab[i] = newNode(hash, key, value, null);
    else {
        Node<K,V> e; K k;
        if (p.hash == hash && ((k = p.key) == key || (key != null && key.equals(k))))
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
                if (e.hash == hash && ((k = e.key) == key || (key != null && key.equals(k))))
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
同样 get() 和 containsKey() 两个方法在内部其实都调用了 getNode() 方法，而在这个方法中判断两个对象是否相等同时使用到了 hashCode() 和 equals() 方法，只有在这两个方法都返回 true 的时候，才会判定这两个对象相同。源码如下：
```java
final Node<K,V> getNode(int hash, Object key) {
    Node<K,V>[] tab; Node<K,V> first, e; int n; K k;
    if ((tab = table) != null && (n = tab.length) > 0 &&
        (first = tab[(n - 1) & hash]) != null) {
        if (first.hash == hash && // always check first node
            ((k = first.key) == key || (key != null && key.equals(k))))
            return first;
        if ((e = first.next) != null) {
            if (first instanceof TreeNode)
                return ((TreeNode<K,V>)first).getTreeNode(hash, key);
            do {
                if (e.hash == hash &&
                    ((k = e.key) == key || (key != null && key.equals(k))))
                    return e;
            } while ((e = e.next) != null);
        }
    }
    return null;
}
```
所以，只需为 Node 类加上 equals() 方法，上面的程序就会输出正确结果：
```java
static class Node<K, V> {
    K key;
    V value;

    Node(K key, V value) {
        this.key = key;
        this.value = value;
    }

    @Override
    public int hashCode() {
        return key.hashCode() * 13 + (value == null ? 0 : value.hashCode());
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o instanceof Node) {
            Node node = (Node) o;
            if (key != null ? !key.equals(node.key) : node.key != null) return false;
            if (value != null ? !value.equals(node.value) : node.value != null) return false;
            return true;
        }
        return false;
    }
}
```