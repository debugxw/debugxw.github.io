---
title: 记一次Golang线上故障排查
excerpt: Package pprof serves via its HTTP server runtime profiling data in the format expected by the pprof visualization tool
date: 2024-03-01
tags: Golang
image: /img/theme/027.jpg
toc: false
---

### 故障现场
本来线上服务跑的好好的，突然有一天，k8s相关同事在微信@我，说我们的服务有问题，经常重启，并给了以下截图，那有啥法，证据都摆在这里了，只能硬着头皮排查了。。。

![](../../../../img/golang/bug/oom_restart.jpg)

![](../../../../img/golang/bug/oom.jpg)

> 既然有了**OOMKilled**这样的报错，那么肯定就是内存相关的问题了!

### 日志、监控
按照正常的排查思路，先来看看日志，根据**oom**或者**panic**这样的关键字，找找看有没有什么关键的报错信息，但是找了半天，发现啥也没有找到，咋办？哦，对了，再来看看服务重启时候的一些接口调用量，先从调用量最多的接口开始看，很有可能就是这些接口造成的。于是乎，开始大致看了一下代码（庆幸的是，接口比较少！！！），却并没有发现什么特别明显的能够造成OOM的地方。咋办呢？看看监控吧

![](../../../../img/golang/bug/oom_monitor.jpg)

从监控中可以发现，在某些情况下，内存和CPU会突然升高，且居高不下。咦？这是咋个回事呢？内存高就算了，怎么CPU也飙的这么高呢？
冷静一下，来分析分析，CPU飙高肯定是做了大量的运算，同时内存飙高。嗯，想到一个非常有可能的原因，那就是死循环，死循环里面做了大量的计算和内存申请。但是，问题又来了，这个项目是刚接手的，也不是很熟悉，去找一个不会百分百复现的死循环，这。。。确实有点难度。。。咋办呢？只有看看能不能把线上的CPU、内存信息给记录下来，如果能拿到这些信息，那不就好办了吗。Google一下，果然有个叫PProf的工具就是用来干这个事情的！

### PProf
PProf是Golang自带的一个性能、数据分析工具，使用起来也很简单，只需要在你的main.go里面添加上如下代码

```golang
package main

import (
	".../log"
	"go.uber.org/zap"
	"net/http"
	_ "net/http/pprof"  // 注意：这个包必须要引入
)

func main() {
	// pprof 采样
	go func() {
		err := http.ListenAndServe("0.0.0.0:8005", nil)
		if err != nil {
			log.Error("pprof_ListenAndServe_err", zap.Error(err))
		} else {
			log.Info("pprof_ListenAndServe_success")
		}
	}()

	// 业务代码
    // ...
}
```

然后我们再下载一个图形化分析工具：graphviz。在控制台执行一下命令即可执行30秒钟的采样：

```go tool pprof -http=:1234 http://127.0.0.1:8005/debug/pprof/profile?seconds=30```

采样结束之后会自动跳转到```localhost:1234/ui/```

![](../../../../img/golang/bug/oom_ui.jpg)

从页面上可以很清楚的看到调用链路以及执行时间，里面有一个很明显的执行时间很长的链路

![](../../../../img/golang/bug/oom_ui_detail.jpg)

从方法名来看，应该是数据库的问题。于是乎找到相应的代码，看了下，发现一行愚蠢至极的代码：

```golang
exceptionUsers, err := exceptionUserRepo.Find(ctx, exceptionUserRepo.Id.NotEq(0))
```

这不就等于查询一张表的所有数据吗。再看看数据库有多少数据，一查，两百多万！！！尼玛，谁教你这么查数据的！！！

> 实际上pprof就是将采样过程中的程序运行情况生成一个profile文件，然后我们就可以通过graphviz这样的图形化分析工具，查看程序的运行情况

**还有一个问题，困扰了我许久，CPU为什么也会飙得这么高呢？**
从数据库查询两百万的数据，基本上是属于I/O操作，内存高可以理解，CPU为什么也会这么高呢？
原来是因为：上面截图里面的**CPU使用量（m）**，这里的单位是m，简单来说1核=1000m，那么16m也就才0.016核，完全可以理解
（主要是一刚开始看到内存和CPU一起飙高，潜意识认为CPU的使用量单位是百分比，就以为CPU100%了。。。）