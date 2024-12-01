---
title: 云原生时代的Java
excerpt: 时代在改变，过往的成就如今却成为了跟上时代的包袱
date: 2023-12-15
tags: Java
image: /img/theme/029.jpg
---

作为一名刚刚从Java转向Golang的程序员，一路走来，总免不了遇到各种语言之间孰好孰坏的话题。刚开始写代码时，我一直认为：语言只是一门工具，真正值得我们学习的是思考问题的抽象思维能力。所以一直也没有对这方面的话题有太多的关注，但随着开发经验的不断积累，的确会发现：在某些特定的场景下，不同的语言实现相同的功能，所需的工作量可能会出现数倍的差距！！！
偶然的一次机会，看到了周志明老师在QCon2020上的一次主题演讲【云原生时代，Java的危与机】。感触颇深，受益良多

### Java的优势
> Write Once, Run Anywhere
  Programming Language Level Virtualization

今年，已经是Java诞生的第26周年，在这26年的时间里，Java也成长为了一门拥有强大生态的语言。Java诞生之初的那句话```Write Once, Run Anywhere```也已经植入了Java的基因。在二十几年前，能够做到平台无关性的语言可不多，而这也大大减轻了程序员的负担（不用再像之前一样，如果想要程序运行在不同的平台、指令集架构上面，通常需要为每一个平台、指令集架构单独编译出一个二进制包；或者不想要这么麻烦的话，索性直接公布源码，其他开发者再来下载源码，然后到自己的平台上面编译打包）

### Java的危机
> Build Once, Run Anywhere
  Operating System Level Virtualization

而在如今的云原生时代，Docker的一句```Build Once, Run Anywhere```，让Java的平台无关性变得好像也不是那么的无可替代。由此，开发者在选择程序的开发语言时，也就有了更多的选择。而如果仅仅是平台无关性的优势减少，也不至于让如今的Java变得危机四伏。更多的原因是那些从Java创始之初就植入在Java基因里面的一些特性、假设，与云原生时代的一些理念发生了直接的冲突！！！

比如，在二十年前，开发者们提倡一个项目应该由少量的大型实例构成，而Java也是为面向大规模、长时间运行的服务端程序而设计。例如能够让程序运行的越来越快的即时编译器、能够在运行时进行内存回收的垃圾收集器、能够让程序在运行时组装新模块的静态类型动态链接的语言结构等，都是为了便于长时间运行的程序能够享受到硬件规模发展的红利。
另一方面，随着微服务的普及，开发者们更愿意将一个项目拆分成多个体量更小的微服务来各司其职，而不是构建一个大型、稳定且能够长期运行的项目。伴随着云原生时代的到来，容器的体积更小、启动速度更快、程序达到最高性能所需要的时间更短、面向失败、具备韧性成为了新的要求，Java原有的优势在云原生上都毫无用武之地

### Java的变革
面对挑战，Java的开发者和社区都没有退缩，它们在各自的领域给出了很多优秀的解决方案，例如[Quarkus](https://quarkus.io/)、[Micronaut](https://micronaut.io/)、[Helidon](https://helidon.io/)等等。Java官方也在积极的推进一些项目，以此来应对云原生时代的到来

![](../../../../img/java/java_change.png)

#### Project Leyden

> Address long-term pain points of Java 's start-up time, time to peak performance and large footprint.

对于原生语言的挑战，最有力最彻底的反击手段无疑是将字节码直接编译成可以脱离Java虚拟机的原生代码。如果真的能够生成脱离Java虚拟机运行的原生程序，将意味着启动时间长的问题能够彻底解决，因为此时已经不存在初始化虚拟机和类加载的过程；也意味着程序马上就能达到最佳的性能，因为此时已经不存在即时编译器运行时编译，所有代码都是在编译期编译和优化好的；没有了Java虚拟机、即时编译器这些额外的部件，也就意味着能够省去它们原本消耗的那部分内存资源与镜像体积。

![Java Performance Matrices](../../../../img/java/project_leyden.png)

如果单单从语言层面去实现将字节码转化为原生代码，是没有什么难度的。这个项目最大的难度在于Java语言从诞生之初就存在的那些特性，比如说类的动态加/卸载、反射、动态代理、字节码生成库等。因为如果想要编译期将代码编译成可以直接运行的原生程序，就意味着在编译期，程序运行所需要的所有类都是已知的。但事实刚好相反，Java本身是一门动态链接的语言，它假设代码的程序空间是开放的，允许在程序的任何时候通过类加载器去加载新的类，作为程序的一部分运行。当然，如果把反射、动态代理等这些基础能力直接抽离掉，HelloWorld肯定也是能够跑起来的，但是Spring肯定跑不起来，大部分的生产工具都跑不起来，整个Java生态中绝大多数上层建筑都会轰然崩塌。

由于Leyden项目刚刚开始，下面来看看SubstrateVM是如何解决这些问题的：
+ 像反射、动态代理这样的基础特性，肯定是不能妥协的，折中的解决办法是以配置文件或者参数的形式告诉编译器，程序中需要用到哪些反射、动态代理、配置、资源等
+ 像动态生成字节码这样的功能，往往程序开发者自己也无法得知那些动态字节码的具体信息，这种就只能由用到CGLib等库的程序去妥协放弃（Spring Framework 5.2开始增加了```@proxyBeanMethods```注解来排除对CGLib的依赖，仅使用标准的动态代理去增强类）

2019年起，Pivotal的Spring团队与Oracle Labs的GraalVM团队共同孵化了Spring GraalVM Native项目，该项目能够让程序先以传统方式运行（启动）一次，自动化地找出程序中的反射、动态代理的代码，代替用户向编译器提供绝大部分所需的信息，并能将允许启动时初始化的Bean在编译期就完成初始化，直接绕过Spring程序启动最慢的阶段，这样从启动到程序可以提供服务，耗时竟能够低于0.1秒

![](../../../../img/java/project_leyden_start_up_time.png)

以原生方式运行后，缩短程序的启动时间的效果立竿见影，一般会有数十倍甚至更高的改善，程序的体积和内存消耗也有一定的下降。但是程序的运行效率和最大吞吐量还是要弱于基于Java虚拟机的方式（因为即时编译器能够进行基于假设的激进优化和运行时性能度量的制导优化，还是要优于提前编译器的）

#### Project Valhalla
> Higher density and performance of machine learning and big data applications introduction of Value Types

由于Java```Everything Is an Object```的假设前提，所有的对象实例都会有一个对象头，倒不是说这个对象头占用了12字节的空间，导致了内存空间的浪费；而是因为：这种假设前提会导致在处理一系列不同类型的小对象时，内存访问性能非常拉胯。这点也是Java在游戏、图形处理等领域一直难有建树的重要制约因素，也是建立Valhalla项目的初衷
举个简单的例子，假设我要在空间中描述一组线段，在Java中，可以这样定义
```java
class Point {
    float x;
    float y;
    float z;
}

class Line {
    Point start;
    Point end;
}

Line[] lines;
```

这组线段的内存布局就应该是这样的

![](../../../../img/java/object_memory_layout.png)

如果我想要访问某条线段的两个点，那么我肯定是**依靠对象标识符来链式访问**。（例如我想要访问line1的两个点，就需要先定位到Header1，然后找到Header2，最后定位到Header3、Header4来访问，这就需要4次内存寻址）

> 面向对象的内存布局中，对象标识符存在的目的是为了允许在不暴露对象结构的前提下，依然可以引用其属性与行为，这是面向对象编程中多态性的基础。在Java中堆内存分配和回收、空值判断、引用比较、同步锁等一系列功能都会涉及到对象标识符

而如果我把这组线段定义成结构体
```golang
type Line struct {
	Start Point
	End   Point
}

type Point struct {
	X int32
	Y int32
	Z int32
}

var lines []Line
```

那么它的内存布局应该是这样的

![](../../../../img/java/struct_memory_layout.png)

可以看到，结构体的内存布局是紧凑的，如果我想要访问line1的两个点，那么我只需要定位到lines数组的内存地址即可访问，内存寻址减少了3次（一次内存访问（将主内存数据调入处理器缓存）大约需要消耗数百个时钟周期，而大部分的简单指令执行只需要一个时钟周期。因此，在程序执行性能这个问题上，如果能减少一次内存访问，往往比优化掉几十、几百条其他指令来得更加划算）。这就是Java的内存访问性能拉垮的原因

从上面的例子中可以看到，将Point和Line都定义成结构体即可减少内存访问。Valhalla项目的核心改进就是提供类似结构体/值类型的支持，提供一个关键字```inline```，让用户可以在不需要向方法外部暴露对象、不需要多态性支持、不需要将对象用作同步锁的场景中，将类标识为```inline```，此时编译器就能绕过对象标识符，以平坦的、紧凑的方式去为对象分配内存

有了```inline```的支持后，现在Java泛型中令人诟病的不支持原生数据类型、频繁装箱问题也就随之迎刃而解，现在Java的包装类，理所当然地会以代表原生类型的值类型来重新定义，这样Java泛型的性能会得到明显的提升，因为此时Integer与int的访问，在机器层面看完全可以达到一致的效率

#### Project Loom
Java抽象了隐藏各种操作系统线程差异性的统一线程接口，这曾经是它区别于其他编程语言的一大优势。不过，统一的线程模型不见得永远都是正确的。Java目前主流的线程模型是一对一的线程模型，这对于CPU密集型的任务很合适，但是对于I/O密集型的任务，这种模型会在线程上下文切换上消耗大量的资源。同时，HotSpot的线程栈容量默认是1MB，线程内核元数据还要消耗额外的2-16KB内存，所以对于Java程序，想要像Golang那样创建数十万的线程（Golang的线程实际上是用户级线程），可谓是举步维艰

Loom的项目目标是让Java支持额外的N:M[线程模型](https://debugxw.github.io/2023/06/26/%E5%8D%8F%E7%A8%8BVS%E7%BA%BF%E7%A8%8B/)，该项目新增加一种**虚拟线程**，本质上是一种有栈协程，多条虚拟线程可以映射到同一条物理线程之中，在用户空间中自行调度，每条虚拟线程的栈容量也可由用户自行决定（和Golang中的协程概念类似）
Loom项目的另一个目标是要尽最大可能保持原有统一线程模型的交互方式。简单来说，原有的Thread、J.U.C、NIO、Future等，在新的线程模型下依然可用，再简单来说，就是恶心的向下兼容。。。

Loom的另一个重点改进是支持**结构化并发（Structured Concurrency）**，这是2016年才提出的新的并发编程概念，但很快就被诸多编程语言所吸纳。它是指程序的并发行为会与代码的结构对齐，譬如以下代码所示
```java
ThreadFactory factory = Thread.builder().virtual().factory();
try (var executor = Executors.newThreadExecutor(factory)) {
    executor.submit(task1);
    executor.submit(task2);
}
// blocks and awaits
```

按照传统的编程观念，这两个任务会被异步执行，程序继续向下执行；如果想要这两个任务都执行完成之后，程序才继续向下执行，那么就需要在类似await这样的关键字或者并发库的帮助下完成。但是在结构化编程的理念中，我们希望原本通过代码关键字或者并发库来实现的并发编程，能够直接和我的代码结构对齐，比如通过上面代码的花括号，在该作用域内，当所有的任务都执行完成之后，才继续执行后面的代码。这样就能用符合人类逻辑思维的代码结构来解决异步编程问题，而不需要额外的关键字或并发库的帮助
> 事实上，```Code like sync, Work like async```正是Loom简化并发编程的核心理念

#### Project Portola
> Optimizing Java for use in containers, including a port of the JDK to Alpine Linux

Portola项目的目标是将OpenJDK向Alpine Linux移植。Alpine Linux是许多Docker容器首选的基础镜像，因为它只有5MB大小，比起其他Cent OS、Debain等动辄一百多MB的发行版来说，更适合用于容器环境。不过Alpine Linux为了尽量瘦身，默认是用musl作为C标准库的，而非传统的glibc，因此要以Alpine Linux为基础制作OpenJDK镜像，必须先安装glibc，此时基础镜像大约有12MB。Portola计划将OpenJDK的上游代码移植到musl，并通过兼容性测试。使用Portola制作的标准Java SE 13镜像仅有41MB，不仅远低于Cent OS的OpenJDK（大约 396 MB），也要比官方的slim版（约 200 MB）要小得多

```shell
$ sudo docker build .
Sending build context to Docker daemon   2.56kB
Step 1/8 : FROM alpine:latest as build
latest: Pulling from library/alpine
bdf0201b3a05: Pull complete
Digest: sha256:28ef97b8686a0b5399129e9b763d5b7e5ff03576aa5580d6f4182a49c5fe1913
Status: Downloaded newer image for alpine:latest
 ---> cdf98d1859c1
Step 2/8 : ADD https://download.java.net/java/early_access/alpine/16/binaries/openjdk-13-ea+16_linux-x64-musl_bin.tar.gz /opt/jdk/
Downloading [==================================================>]  195.2MB/195.2MB
 ---> Using cache
 ---> b1a444e9dde9
Step 3/7 : RUN tar -xzvf /opt/jdk/openjdk-13-ea+16_linux-x64-musl_bin.tar.gz -C /opt/jdk/
 ---> Using cache
 ---> ce2721c75ea0
Step 4/7 : RUN ["/opt/jdk/jdk-13/bin/jlink", "--compress=2",      "--module-path", "/opt/jdk/jdk-13/jmods/",      "--add-modules", "java.base",      "--output", "/jlinked"]
 ---> Using cache
 ---> d7b2793ed509
Step 5/7 : FROM alpine:latest
 ---> cdf98d1859c1
Step 6/7 : COPY --from=build /jlinked /opt/jdk/
 ---> Using cache
 ---> 993fb106f2c2
Step 7/7 : CMD ["/opt/jdk/bin/java", "--version"] - to check JDK version
 ---> Running in 8e1658f5f84d
Removing intermediate container 8e1658f5f84d
 ---> 350dd3a72a7d
Successfully built 350dd3a72a7d

$ sudo docker tag 350dd3a72a7d jdk-13-musl/jdk-version:v1

$ sudo docker images
REPOSITORY                TAG                 IMAGE ID            CREATED              SIZE
jdk-13-musl/jdk-version   v1                  350dd3a72a7d        About a minute ago   41.7MB
alpine                    latest              cdf98d1859c1        2 weeks ago          5.53MB
```

### 结束语
云原生时代，Java技术体系的许多前提假设都受到了挑战
+ 一次编译，到处运行
+ 面向长时间大规模程序而设计
+ 从开放的代码空间中动态加载
+ 一切皆为对象
+ 统一线程模型
+ ...

技术发展迭代不会停歇，没有必要坚持什么“永恒的真理”，旧的原则被打破，只要合理，便是创新

Java语言意识到了挑战，也意识到了要面向未来而变革。Java的未来是继续向前，再攀高峰，还是由盛转衰，锋芒挫缩，你我拭目以待

> 参考
[云原生时代的Java](https://time.geekbang.org/opencourse/detail/100067401)
[QCon2020 主题演讲：云原生时代，Java 的危与机](https://icyfenix.cn/tricks/2020/java-crisis/qcon.html)