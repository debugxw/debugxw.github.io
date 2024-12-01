---
title: 三个线程顺序打印ABC
excerpt: volatile关键字的巧妙运用
date: 2020-09-19
tag: [并发,奇思妙想]
toc: true
image: /img/theme/002.jpg
---

## 用Java实现三个线程顺序打印ABC

这是一道典型的面试题，之前在面试的时候就被问到过，但当时的实现方式很low，记得面试官的评价是“嗯。。。也行”。前两天QFC作业题又遇到了，第一反应是用Reentrantlock和Condition来实现，如下：

```java
public class PrintInOrder {
 
    private static Lock lock = new ReentrantLock();
    private static Condition waitA = lock.newCondition();
    private static Condition waitB = lock.newCondition();
    private static Condition waitC = lock.newCondition();
 
    public static void main(String[] args) throws InterruptedException {
 
        new Thread(new Task('A', lock, waitC, waitA)).start();
        new Thread(new Task('B', lock, waitA, waitB)).start();
        new Thread(new Task('C', lock, waitB, waitC)).start();
        // 控制哪一个线程最先开始执行，Condition.await()和notify()有先后顺序
        // 保证主线程在Thread-A调用waitC.await()之后执行waitC.signal()
        Thread.sleep(10);
        lock.lock();
        try {
            waitC.signal();
        } finally {
            lock.unlock();
        }
    }
 
    private static class Task implements Runnable {
 
        private static final int PRINT_TIMES = 6;
 
        private char printChar;
 
        private Lock lock;
 
        private Condition conditionAwait;
 
        private Condition conditionNotify;
 
        @Override
        public void run() {
            for (int printTimes = 0; printTimes < PRINT_TIMES; printTimes++) {
                lock.lock();
                try {
                    try {
                        conditionAwait.await();
                    } catch (InterruptedException e) {
                        e.printStackTrace();
                    }
                    System.out.print(printChar);
                    conditionNotify.signal();
                } finally {
                    lock.unlock();
                }
            }
        }
 
        Task(char printChar, Lock lock, Condition conditionAwait, Condition conditionNotify) {
            this.printChar = printChar;
            this.lock = lock;
            this.conditionAwait = conditionAwait;
            this.conditionNotify = conditionNotify;
        }
    }
}
```

但是这种实现方式有一个问题，因为Condition的await()和signal()方法有先后顺序之分，所以要保证主线程在Thread-A调用waitC.await()之后执行waitC.signal()，所以在主线程里会sleep一下，来等待A线程执行到waitC.await()。当然也可以在Task的run()中进行判断，如果是A线程且是第一次打印，那么就不调用waitC.await()直接打印，但是这样的话会使Task的功能变得不那么纯粹，如果这种判断条件变多，那么代码逻辑将会变得非常混乱

## 另一种比较好的实现方式

```java
public class PrintInOrderOptimize {
 
    public static void main(String[] args) {
        Task taskA = new Task('A');
        Task taskB = new Task('B');
        Task taskC = new Task('C');
 
        Thread threadA = new Thread(taskA);
        Thread threadB = new Thread(taskB);
        Thread threadC = new Thread(taskC);
 
        taskA.setUnparkThread(threadB);
        taskB.setUnparkThread(threadC);
        taskC.setUnparkThread(threadA);
 
        threadA.start();
        threadB.start();
        threadC.start();
 
        // 相当于给A线程一个许可permit，而不用考虑线程A调用LockSupport.park()是在此之前，还是在此之后
        LockSupport.unpark(threadA);
    }
 
    private static class Task implements Runnable {
 
        private static final int PRINT_TIMES = 6;
 
        private char printChar;
 
        @Setter
        private Thread unparkThread;
 
        Task(char printChar) {
            this.printChar = printChar;
        }
 
        @Override
        public void run() {
            for (int i = 0; i < PRINT_TIMES; i++) {
                LockSupport.park();
                System.out.print(printChar);
                LockSupport.unpark(unparkThread);
            }
        }
    }
}
```

这种实现方式能够弥补第一种方式的不足，主要是利用了LockSupport.park()和LockSupport.unpark()不用考虑使用的先后顺序

## 另一种脑壳好用才能想得出来的实现方式

因为该题目是要顺序打印ABC，也就是B要等A先打印，C要等B先打印，A要等C先打印（当然打印第一个A除外）。三个线程一个一个的打印，也就是相当于同一时间只有一个线程在打印。所以就可以只用一个volatile变量来控制打印顺序

```java
public class PrintInOrderAmaz {
 
    private static final int PRINT_TIMES = 6;
 
    // 这里有个很奇葩的问题，如果将volatile关键字去掉，将PRINT_TIMES设大一点如500，那么三个线程会卡住？？？
    private static volatile char flag = 'A';
 
    private static void print(char printChar) {
        int times = 0;
        while (times < PRINT_TIMES) {
            // 三个线程同时读flag，但是不存在同时写操作（对于flag的写操作，同一时间只会有一个线程在执行<printChar == flag 为 true 的线程>）
            // 所以volatile的可见性就可以保证读数据的正确性
            if (printChar == flag) {
                System.out.print(printChar);
                flag = flag == 'C' ? 'A' : (char) (flag + 1);
                times++;
            }
        }
    }
 
    public static void main(String[] args) {
 
        new Thread(() -> print('A')).start();
        new Thread(() -> print('B')).start();
        new Thread(() -> print('C')).start();
    }
}
```