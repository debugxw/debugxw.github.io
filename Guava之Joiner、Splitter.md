---
title: Guava之Joiner、Splitter
excerpt: 字符串操作工具类，提供了连接字符串和分割字符串的简便方式
date: 2020-09-27
tags: [Java,设计模式]
image: /img/theme/025.jpg
---

### 一、Joiner—继承的典型应用

#### 1、基本用法

```java
List<Integer> integers = Lists.newArrayList(1, 2, null, 3);
String res = Joiner.on("-").skipNulls().join(integers);
// 输出 1-2-3
```

#### 2、Joiner.join()方法

join()内部会调用appendTo()方法，该方法也是最核心的实现

```java
@CanIgnoreReturnValue
public <A extends Appendable> A appendTo(A appendable, Iterator<?> parts) throws IOException {
  /**
   * 刚开始看的时候觉得，如果appendable为空，后面对于他的调用不是一样会抛出异常吗？为什么还要提前在
   * 这里调用Preconditions.checkNotNull()进行检查呢？
   * 原因：
   *    我们期望的是尽早抛出异常，而不是等到数据被层层传递，传递到非常深的位置，这样不仅浪费系统资源
   *    更不利于我们精确的定位错误。所以推荐在方法的入口，或运算开始前，先检查数据。
   *    例如：
   *        int length = datas.get(5).getDescription().concat("XX").split(",").length;
   *    像这种代码，就算抛出空指针也不知道到底是哪个对象空指针了，需要一步一步调试。
   *    这就是Preconditions另一个作用：尽早发现错误，精确控制出错的位置。
   */
  checkNotNull(appendable);
  if (parts.hasNext()) {
    appendable.append(toString(parts.next()));
    while (parts.hasNext()) {
      appendable.append(separator);
      appendable.append(toString(parts.next()));
    }
  }
  return appendable;
}
```

#### 3、Joiner.toString()方法

```java
CharSequence toString(Object part) {
  // 该方法会检查待连接的对象part是否为空，在这里，如果part为空那么会抛出异常
  // 也就说，默认情况下对于待连接的对象不能为空
  checkNotNull(part); // checkNotNull for GWT (do not optimize).
  return (part instanceof CharSequence) ? (CharSequence) part : part.toString();
}
```

#### 4、Joiner.useForNull()方法

该方法会用指定的字符串代替待连接Iterator中的null

```java
// 这里是重点，useForNull会返回一个Joiner的子类，并重写toString,useForNull,skipNulls三个方法
public Joiner useForNull(final String nullText) {
  checkNotNull(nullText);
  return new Joiner(this) {
    /**
     * 重写toString方法相当于是一个拦截器，因为是重写Joiner的toString方法，所以在appendTo方法中调
     * 用的toString方法就会是这个方法，那么该方法就会用指定的nullText来代替待连接Iterator中的null
     * 这是将java继承利用得很好的一个实例，思路也很好，写这个代码的人@author Kevin Bourrillion，真牛皮！！！
     */
    @Override
    CharSequence toString(@Nullable Object part) {
      // 这里的Joiner.this.toString()方法调用的是com.google.common.base.Joiner.toString()方法
      // 这里的语法是，内部类调用外部类的方法
      return (part == null) ? nullText : Joiner.this.toString(part);
    }
 
    // 正是由于这个重写方法，一个Joiner的useForNulls只能调用一次。例如，下面的代码就会抛异常
    // Joiner.on("-").useForNull("#").useForNull("*").join(integers)
    @Override
    public Joiner useForNull(String nullText) {
      throw new UnsupportedOperationException("already specified useForNull");
    }
 
    // 和useForNull()方法类似，该方法直接抛异常，也就是useForNull()和skipNulls()两个方法不能同时使用
    @Override
    public Joiner skipNulls() {
      throw new UnsupportedOperationException("already specified useForNull");
    }
  };
}
```

#### 5、Joiner.skipNulls()方法

```java
public Joiner skipNulls() {
  return new Joiner(this) {
    @Override
    public <A extends Appendable> A appendTo(A appendable, Iterator<?> parts) throws IOException {
      checkNotNull(appendable, "appendable");
      checkNotNull(parts, "parts");
      while (parts.hasNext()) {
        Object part = parts.next();
        // skipNulls()能忽略null核心逻辑就是这个if语句和下一个if
        if (part != null) {
          appendable.append(Joiner.this.toString(part));
          break;
        }
      }
      while (parts.hasNext()) {
        Object part = parts.next();
        // important
        if (part != null) {
          appendable.append(separator);
          appendable.append(Joiner.this.toString(part));
        }
      }
      return appendable;
    }
 
    // 不能和useForNull()一起使用
    @Override
    public Joiner useForNull(String nullText) {
      throw new UnsupportedOperationException("already specified skipNulls");
    }
 
    // 不能和withKeyValueSeparator()一起使用
    @Override
    public MapJoiner withKeyValueSeparator(String kvs) {
      throw new UnsupportedOperationException("can't use .skipNulls() with maps");
    }
  };
}
```

#### 6、Joiner.withKeyValueSeparator()方法

```java
// 该方法会返回一个MapJoiner，MapJoiner内部会实现join(),appendTo()，useForNull()等方法
public MapJoiner withKeyValueSeparator(String keyValueSeparator) {
  // MapJoiner内部有两个成员变量
  // private final Joiner joiner;
  // private final String keyValueSeparator;
  // 这里使用调用此方法的joiner来构造MapJoiner
  return new MapJoiner(this, keyValueSeparator);
}
```

#### 7、MapJoiner.appendTo()方法

```java
@Beta
@CanIgnoreReturnValue
public <A extends Appendable> A appendTo(A appendable, Iterator<? extends Entry<?, ?>> parts)
    throws IOException {
  checkNotNull(appendable);
  if (parts.hasNext()) {
    Entry<?, ?> entry = parts.next();
    // 实现原理和Joiner.appendTo()方法大同小异，只不过这里多了两个值
    appendable.append(joiner.toString(entry.getKey()));
    appendable.append(keyValueSeparator);
    appendable.append(joiner.toString(entry.getValue()));
    while (parts.hasNext()) {
      appendable.append(joiner.separator);
      Entry<?, ?> e = parts.next();
      // 一样的
      appendable.append(joiner.toString(e.getKey()));
      appendable.append(keyValueSeparator);
      appendable.append(joiner.toString(e.getValue()));
    }
  }
  return appendable;
}
```

#### 8、MapJoiner.useForNnull()方法

```java
// 可以看到Mapjoiner.appendTo()方法中有如下代码
// appendable.append(joiner.toString(entry.getKey()));
// 这里的joiner就是在Joiner.withKeyValueSeparator()方法中传入的joiner，直接调用该joiner的useForNull()方法
// 就会返回一个特定的joiner，然后再用它来构造Mapjoiner，那么在MapJoiner.appendTo()中就可以直接调用joiner.toString()方法
public MapJoiner useForNull(String nullText) {
  return new MapJoiner(joiner.useForNull(nullText), keyValueSeparator);
}
```

### 二、Splitter—策略模式和Iterator的使用

#### 1、Splitter.Strategy

Splitter内部使用了策略模式来解决Splitter的分隔符可以为字符、字符串、正则表达式的问题。网上说策略模式主要解决，在有多种算法相似的情况下，使用 if…else 所带来的复杂性和难以维护性。但看了几个例子感觉策略模式就是面向接口编程，同一个类的对象对于其相同的行为有不同的实现。。。

```java
private interface Strategy {
    Iterator<String> iterator(Splitter splitter, CharSequence toSplit);
}
```

#### 2、内部成员

```java
// 移除指定字符项，即集合中当前元素与trimmer匹配，将其移除。如果没有设置trimmer，则将结果中的空格删除
// 最终结论为：将结果集中的每个字符串前缀和后缀都去除trimmer，直到前缀或后缀没有这个字符了，字符串“中”的不用去除
private final CharMatcher trimmer;
 
// 是否移除结果集中的空集，true为移除结果集中的空集，false为不用移除结果集中的空集
private final boolean omitEmptyStrings;
 
 
// 这个变量最终会返回一个所有集合类的父接口，它是贯穿着整个字符串分解的变量
// 该变量就是上面Strategy接口的具体实现
private final Strategy strategy;
 
 
// 最多将字符串分为几个集合
private final int limit;
```

#### 3、惰性迭代器Splitter.SplittingIterator

该类是一个惰性迭代器，直到不得不计算的时候才会去将字符串分割，即在迭代的时候才去分割字符串，无论将分隔符还是被分割的字符串加载到Splitter类中，都不会去分割，只有在迭代的时候才会真正的去分割

```java
private abstract static class SplittingIterator extends AbstractIterator<String> {
  final CharSequence toSplit;
  final CharMatcher trimmer;
  final boolean omitEmptyStrings;
 
  // 获取被分割字符串中第一个与分隔符匹配的起始位置
  abstract int separatorStart(int start);
 
  // 获取被分割字符串中第一个与分隔符匹配的结束位置，与上面的start是左闭右开的关系[start, end)
  abstract int separatorEnd(int separatorPosition);
 
  int offset = 0;
  int limit;
 
  protected SplittingIterator(Splitter splitter, CharSequence toSplit) {
    this.trimmer = splitter.trimmer;
    this.omitEmptyStrings = splitter.omitEmptyStrings;
    this.limit = splitter.limit;
    this.toSplit = toSplit;
  }
 
  // SplittingIterator类继承自AbstractIterator。AbstractIterator是一个抽象类，它实现了Iterator<T>接口。
  // AbstractIterator抽象类是为了让Iterator接口对于某些类型的数据源更易于实现而封装的
  // Iterator接口要求其实现支持使用hasNext()方法查询数据结束状态，而无需更改迭代器的状态。
  // 但是，许多数据源没有公开此信息。这些类型的数据源通常很难为其编写迭代器。
  // 但是使用此类，可以直接重写computeNext()方法并在适当时调用endOfData()方法。
 
 
  // 重写迭代方法，Splitter分割字符串的具体实现
  @Override
  protected String computeNext() {
    int nextStart = offset;
    while (offset != -1) {
      int start = nextStart;
      int end;
 
      int separatorPosition = separatorStart(offset);
      if (separatorPosition == -1) {
        // 当前[start, end)是字符串分割之后的最后一个元素
        end = toSplit.length();
        offset = -1;
      } else {
        end = separatorPosition;
        offset = separatorEnd(separatorPosition);
      }
      if (offset == nextStart) {
        // 该结果集为空，那么直接进入下一个循环
        offset++;
        if (offset > toSplit.length()) {
          offset = -1;
        }
        continue;
      }
 
      // 如果结果集中字符串的前缀包含trimer，那么去除
      // 这里的trimer是guava提供的CharMatcher实例，这个是用来匹配字符的，而不是匹配字符串
      // 例如：
      //        Splitter.on("ell").trimResults(CharMatcher.anyOf("ab").or(CharMatcher.anyOf("ad"))).split("abhellddxdo")
      //        结果为：[h, xdo]
      while (start < end && trimmer.matches(toSplit.charAt(start))) {
        start++;
      }
      // 如果结果集中字符串的后缀包含trimer，那么去除
      while (end > start && trimmer.matches(toSplit.charAt(end - 1))) {
        end--;
      }
 
      // 该结果集元素为空，如果omitEmptyStrings，那么直接跳过
      if (omitEmptyStrings && start == end) {
        nextStart = offset;
        continue;
      }
 
      if (limit == 1) {
        // 结果集个数达到上限
        // 该操作在结果集元素判空之后进行，所以空串不会进行计数
        end = toSplit.length();
        offset = -1;
        while (end > start && trimmer.matches(toSplit.charAt(end - 1))) {
          end--;
        }
      } else {
        limit--;
      }
 
      // 剪切字符串并返回
      return toSplit.subSequence(start, end).toString();
    }
    return endOfData();
  }
}
```

#### 4、Splitter.on()方法

```java
// 分割符为字符串
public static Splitter on(final String separator) {
  checkArgument(separator.length() != 0, "The separator may not be the empty string.");
  if (separator.length() == 1) {
    return Splitter.on(separator.charAt(0));
  }
  // 直接返回一个包含具体策略Strategy的Splitter
  return new Splitter(
      new Strategy() {
        // 重写具体分割Iterator
        @Override
        public SplittingIterator iterator(Splitter splitter, CharSequence toSplit) {
          return new SplittingIterator(splitter, toSplit) {
            @Override
            public int separatorStart(int start) {
              int separatorLength = separator.length();
 
              positions:
              for (int p = start, last = toSplit.length() - separatorLength; p <= last; p++) {
                for (int i = 0; i < separatorLength; i++) {
                  if (toSplit.charAt(i + p) != separator.charAt(i)) {
                    continue positions;
                  }
                }
                return p;
              }
              return -1;
            }
 
            @Override
            public int separatorEnd(int separatorPosition) {
              return separatorPosition + separator.length();
            }
          };
        }
      });
}
```

#### 5、Splitter.omitEmptyStrings()方法

```java
// 去除结果集中的空字符串
public Splitter omitEmptyStrings() {
  // 仅仅是将omitEmptyStrings变量设置为true，然后核心处理逻辑还是在SplittingIterator.computeNext()方法中
  return new Splitter(strategy, true, trimmer, limit);
}
```

#### 6、Splitter.trimResults()方法

```java
// 删除结果集各元素前后的空格，仅仅将trimmer设置为CharMatcher.whitespace()
// 核心处理逻辑还是在SplittingIterator.computeNext()方法中
public Splitter trimResults() {
  return trimResults(CharMatcher.whitespace());
}
 
 
// 删除结果集各元素前后的指定字符串
public Splitter trimResults(CharMatcher trimmer) {
  checkNotNull(trimmer);
  return new Splitter(strategy, omitEmptyStrings, trimmer, limit);
}
```

#### 7、Splitter.limit()方法

```java
// 设置结果集元素的最大个数，仅仅将limit设置为maxItems即可，核心处理逻辑还是在SplittingIterator.computeNext()方法中
public Splitter limit(int maxItems) {
  checkArgument(maxItems > 0, "must be greater than zero: %s", maxItems);
  return new Splitter(strategy, omitEmptyStrings, trimmer, maxItems);
}
```

#### 8、Splitter.withKeyValueSeparator()方法

```java
// 该方法会最终返回一个MapSplitter实例
public MapSplitter withKeyValueSeparator(String separator) {
  return withKeyValueSeparator(on(separator));
}
 
 
public MapSplitter withKeyValueSeparator(Splitter keyValueSplitter) {
  return new MapSplitter(this, keyValueSplitter);
}
 
 
private MapSplitter(Splitter outerSplitter, Splitter entrySplitter) {
  this.outerSplitter = outerSplitter; // only "this" is passed
  this.entrySplitter = checkNotNull(entrySplitter);
}
```

#### 9、MapSplitter类

```java
// 在MapSplitter中有两个成员变量，Splitter，实际上这两个Splitter就是分割字符串为单个entry和将单个entry分割为key-value键值对的Splitter
public static final class MapSplitter {
  private static final String INVALID_ENTRY_MESSAGE = "Chunk [%s] is not a valid entry";
  // 将带分割字符串分割为单个entry的Splitter
  private final Splitter outerSplitter;
  // 将单个entry分割为key-value键值对的Splitter
  private final Splitter entrySplitter;
 
  private MapSplitter(Splitter outerSplitter, Splitter entrySplitter) {
    this.outerSplitter = outerSplitter; // only "this" is passed
    this.entrySplitter = checkNotNull(entrySplitter);
  }
 
 
  ...
}
```

#### 10、MapSplitter.split()方法

```java
// MapSplitter将字符串分割为Map的核心逻辑
public Map<String, String> split(CharSequence sequence) {
  Map<String, String> map = new LinkedHashMap<>();
  // 这里先将待分割字符串分割为单个entry
  for (String entry : outerSplitter.split(sequence)) {
    // 再将单个entry分割为key-value键值对
    Iterator<String> entryFields = entrySplitter.splittingIterator(entry);
 
    checkArgument(entryFields.hasNext(), INVALID_ENTRY_MESSAGE, entry);
    String key = entryFields.next();
    // 如果key已存在，抛异常，所以如果想要将字符串分割为Map，那么其中的key不能重复
    checkArgument(!map.containsKey(key), "Duplicate key [%s] found.", key);
 
    checkArgument(entryFields.hasNext(), INVALID_ENTRY_MESSAGE, entry);
    String value = entryFields.next();
    map.put(key, value);
    // 从上面的几个checkArgument可以看到，MapSplitter对待分割字符串有着严格的格式检查，如果格式不合法，直接抛异常
 
    checkArgument(!entryFields.hasNext(), INVALID_ENTRY_MESSAGE, entry);
  }
  return Collections.unmodifiableMap(map);
}
```

这就是MapSplitter的核心实现，其实就是利用了Splitter，一个Splitter先将字符串分割为单个entry，另一个Splitter再将单个entry分割为key-value键值对。仔细想来，这的确是一种很好的复用方式，这种逻辑思维很值得学习

之前考虑过一个问题，on()方法和withKeyValueSeparator()方法的使用有先后顺序吗？如果先用withKeyValueSeparator()，后用on()会怎样？
后来才发现on()是一个静态方法，而withKeyValueSeparator()方法是非静态的。而Splitter的构造函数是private的，所以想得到一个Splitter实例，就只能调用on()方法，然后才能调用其他方法，那么上面的问题就迎刃而解了，可以发现omitEmptyStrings()、trimResults()、limit()等方法都是非静态的，所以就只能在on()方法调用之后调用！！！！机智