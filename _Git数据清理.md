---
title: Git数据清理
excerpt: 优化存储空间、提升操作效率、确保团队协作的顺畅性
<!-- 2025-02-05 -->
date: 2023-02-05
tags: 存储,工具
toc: true
image: /img/theme/030.jpg
---

前段时间在一个新项目中，把项目的可执行程序放进了git仓库中，虽然占用空间不是很大，但还是想着把它给删除掉。Google一番之后，发现大多文章写的让人云里雾里，索性先来整理下Git内部原理

## Git原理分析
### 存储机制
从根本上来讲Git是一个内容寻址（content-addressable）文件系统，并在此之上提供了一个版本控制系统。其核心部分是一个简单的键值对数据库（位于```.git/objects```目录下），我们可以向Git仓库中插入任意类型的内容，它会返回一个唯一的键，通过该键可以在任意时刻再次取回该内容

#### 数据对象
当我们执行commit之后，Git会以本次修改内容做一系列的哈希运算得到的40位哈希值的后38位作为文件名，前两位作为目录，将本次的所有修改过的文件内容存储在```.git/objects```目录下

![对象存储路径]()

用于存储文件数据的对象即为数据对象（Git称之为Blog），可以通过```git cat-file -t```命令来查看对象的类型(注意：需要提供40位的哈希值)

![数据对象]()

#### 树对象
对于数据对象，其中只存储了每个文件的数据，但是却并没有存储数据属于哪个文件，也就是我们需要一个地方来存储文件名、文件的目录结构等信息。这就需要用到树对象（Git称之为Tree），一个树对象包含了一条或多条树对象记录，每条记录含有一个指向数据对象或者子树对象的指针，以及相应的模式、类型、文件名信息等。可以通过```git cat-file -p release^{tree}```命令来查看release分支上最新的提交所指向的树对象

![树对象（该树对象保存了0个指向Blob对象的指针和0个指向Tree对象的指针）]()

简单来说，Git内部存储的数据类似于

![存储结构]()

这样，我们就可以通过一个树对象获取到仓库所有文件，简单理解，一个树对象即为该时间点Git仓库的全量快照。再进一步，如果能将这些树对象以链表的形式连接起来，不就可以实现版本控制了么

#### 提交对象
提交对象（Git称之为Commit）即是将树对象连接起来实现版本控制的大功臣。提交对象的格式很简单：它先指定一个顶层树对象，代表当前项目快照；然后是可能存在的父提交；之后是作者/提交者信息（依据你的user.name和user.email配置来设定，外加一个时间戳）；最后是提交注释。
至此，我们得到了一个大致完整的Git数据存储结构

![存储结构]()

##### 全量快照存储
在我们日常使用Git的过程中，经常会通过UI工具对比不同版本之间的差异，这也很容易造成一个误区：Git对于不同版本的数据是增量存储（即每次提交至保存与上一次提交的差异）。但实际上，对于不同版本的数据Git存储的都是全量的数据，这样有一个明显好处是，在切换版本时不需要逐层计算差异，速度会快上许多。但也有一个显而易见的缺点，存储空间占用较大，当然Git对其也做了一些优化：
+ 如果某个文件在上次提交后没有变化，Git不会重新存储该文件的内容，而是复用已有的Blob对象
+ Blob对象都是使用zlib算法对文件压缩后得到的(经过实测，一个105K的文本文件，zlib压缩之后的Blob文件只有4.5K)

> Tips
曾经有一个困扰我很久的问题，例如一个仓库所有文件一共有10MB，我把这些文件全部删除之后，再提交一个版本，仓库下什么东西都没有了，但是却依然能够回退到上一个10MB的版本，为什么？后来知道了，是因为版本数据是存储在.git目录下的。但是新的问题又来了，我放了一个100MB的文本文件在仓库里，即然版本数据是放在.git目录下的，为什么.git目录占用的空间远小于100MB，现在知道了，原来Git会使用zlib对文件进行压缩（同时还将对象压缩为空间占用更小的包文件）

##### 引用
通过提交对象，我们可以在不同版本之间进行切换，只要你记得每个版本的40位哈希值，不过这可能会有点难。如果我们有一个文件来保存这个哈希值，而该文件有一个简单的名字，然后用这个名字指针来替代原始的哈希值的话会更加简单。在Git中，这种简单的名字被称为“引用”，你可以在```.git/refs```目录下找到这类含有哈希值的文件。这基本就是Git分支的本质：一个指向某一系列提交之首的指针或引用
至此，Git存储结构从概念上看起来是这样的

![概念存储结构]()

#### 标签对象
标签对象（Git称之为Tag）非常类似于一个提交对象——它包含一个标签创建者信息、一个日期、一段注释信息，以及一个指针。主要的区别在于，标签对象通常指向一个提交对象，而不是一个树对象。它像是一个永不移动的分支引用——永远指向同一个提交对象，只不过给这个提交对象加上一个更友好的名字罢了

### 压缩机制
到这里，我们知道Git管理版本的方式是存储不同版本的全量快照。我们执行的Git命令，也几乎是在往Git数据库中添加数据，即使是删除了某个文件，对于Git来说也是新增存储对象。所以Git仓库会变得越来越大也就容易理解了，但是Git仓库就这样也来越大，而Git却什么都做不了吗？显然不会

Git最初向磁盘中存储对象时所使用的格式被称为松散对象格式。但是，Git会时不时地将多个这些对象打包成一个称为包文件的二进制文件，以节省空间和提高效率。当版本库中有太多的松散对象，或者你手动执行git gc命令，或者你向远程服务器执行推送时，Git都会这样做。举个例子来说明
```
➜ git curl https://raw.githubusercontent.com/mojombo/grit/master/lib/grit/repo.rb > repo.rb
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 22044  100 22044    0     0  28369      0 --:--:-- --:--:-- --:--:-- 28554

git ✗ ➜ git add --all

git ✗ ➜ git commit -m "add repo.rb"
[master（根提交） 8ccf012] add repo.rb
 1 file changed, 709 insertions(+)
 create mode 100644 repo.rb

➜ git cat-file -p master^{tree}
100644 blob 033b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5	repo.rb

➜ ll -h .git/objects/03/3b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5
-r--r--r--@ 1 edy  staff   6.7K  2  7 11:07 .git/objects/03/3b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5

➜ git cat-file -s 033b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5
22044

➜ find .git/objects -type f
.git/objects/03/3b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5
.git/objects/38/feecbdf638935287fd920e8f2d694aa8c28d9f
.git/objects/8c/cf012107dcbe00554cb2af2c42ed35920fea8d
```

首先我们往Git仓库里面放了一个大小约为22K的文件，并提交生成一个版本V1。这时我们可以看到Git生成了一个哈希值为033b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5的Blob对象，该对象大小约为6K（注意：```git cat-file -s```展示的是Blob对象指向的文件的大小，而不是Blob对象本身的大小）(objects目录下的其他两个对象类型分别为Tree和Commit)

接着我们来修改一下repo.rb文件的内容，并提交生成版本V2
```
➜ echo 'add something' >> repo.rb

git ✗ ➜ git commit -am "add something to repo.rb"
[master 91b2f7a] add something to repo.rb
 1 file changed, 1 insertion(+)

➜ git cat-file -p master^{tree}
100644 blob 03c9e1d193bb52be4c31569471ddc641d04ec84b	repo.rb

➜ ll -h .git/objects/03/c9e1d193bb52be4c31569471ddc641d04ec84b
-r--r--r--@ 1 edy  staff   6.7K  2  7 11:21 .git/objects/03/c9e1d193bb52be4c31569471ddc641d04ec84b

➜ git cat-file -s 03c9e1d193bb52be4c31569471ddc641d04ec84b
22058

➜ find .git/objects -type f
.git/objects/03/c9e1d193bb52be4c31569471ddc641d04ec84b
.git/objects/03/3b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5
.git/objects/38/feecbdf638935287fd920e8f2d694aa8c28d9f
.git/objects/91/b2f7a18244790ef199c94ac2308408ac47a0f8
.git/objects/de/ae617f7cd38b4cff9d7d8b775197b59c8a57ca
.git/objects/8c/cf012107dcbe00554cb2af2c42ed35920fea8d
```

可以看到，Git在版本V2中又重新生成了一个和V1完全不同的Blob对象，哈希值为03c9e1d193bb52be4c31569471ddc641d04ec84b，很显然这样会占用大量的存储空间。我们可以手动执行```git gc```命令让Git对对象进行打包，来看看能节约多大的存储空间
```
➜ find .git/objects -type f
.git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.idx
.git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.pack
.git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.rev
.git/objects/info/commit-graph
.git/objects/info/packs

➜ ll -h .git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.pack
-r--r--r--@ 1 edy  staff   6.1K  2  7 11:25 .git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.pack
```

可以看到，```.git/objects```目录下原来的对象不见了，但是在```.git/objects/pack```下却多了以pack、idx为后缀的文件，这就是打包生成的包文件和一个索引。包文件包含了刚才从文件系统中移除的所有对象的内容。索引文件包含了包文件的偏移信息，我们通过索引文件就可以快速定位任意一个指定对象。有意思的是运行gc命令前磁盘上的对象大小约为13K(6.7K+6.7K)，而这个新生成的包文件大小仅有6.1K。通过打包对象减少了一半的磁盘占用空间

Git是如何做到这点的？Git打包对象时，会查找命名及大小相近的文件，并只保存文件不同版本之间的差异内容。你可以查看包文件，观察它是如何节省空间的。```git verify-pack```这个底层命令可以让你查看已打包的内容
```
➜ git verify-pack -v .git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.pack
91b2f7a18244790ef199c94ac2308408ac47a0f8 commit 217 155 12
8ccf012107dcbe00554cb2af2c42ed35920fea8d commit 156 114 167
03c9e1d193bb52be4c31569471ddc641d04ec84b blob   22058 5801 281
deae617f7cd38b4cff9d7d8b775197b59c8a57ca tree   35 46 6082
38feecbdf638935287fd920e8f2d694aa8c28d9f tree   35 46 6128
033b4468fa6b2a9547a70d88d1bbe8bf3f9ed0d5 blob   9 20 6174 1 03c9e1d193bb52be4c31569471ddc641d04ec84b
非 delta：5 个对象
链长 = 1: 1 对象
.git/objects/pack/pack-5e6e54b3e45290c8a763c83b308431a063577448.pack: ok
```

此处，033b4这个数据对象（即repo.rb文件的版本1）引用了数据对象03c9e，即该文件的版本2。命令输出内容的第四列显示的是各个对象在包文件中的大小，可以看到03c9e占用了5.8K空间，而033b4仅占用20字节。同样有趣的地方在于，第二个版本完整保存了文件内容，而原始的版本反而是以差异方式保存的——这是因为大部分情况下需要快速访问文件的最新版本

### Git VS SVN
前文提到对于不同版本的数据Git存储的都是全量的数据，这样有一个明显好处是，在切换版本时不需要逐层计算差异，速度会快上许多。但在上面介绍了包文件的差异/增量存储之后，不禁会生成一个疑问：如果没有包文件，那Git的全量存储以及高性能的历史回溯毋庸置疑。但是既然有了包文件并且它的存储机制是增量存储，那在切换历史版本时，不一样需要逐层计算差异吗？就历史回溯这一点来说，Git相比于SVN性能上的优势体现在哪里呢？关键在于一下两点

#### 回溯链路短
+ Git
  + 例如有四个版本v1->v2->v3->v4，我们需要回溯到v2，只需要解析v2、v3、v4，而不需要解析历史所有版本
  + 增量深度可控，```git repack --depth=50```可以设置将增量链的最大深度限制为50
    假设一个文件有100个版本，且设置--depth=50。Git在打包时会自动分割成两个增量链v1->v50和v51->v100，每个链的末尾对象v50和v100会被存储为完整对象，而非差异。当我们需要从v100回溯到v1时，由于Tree对象的存在，我可以直接定位到v50，然后从v50解析到v1，这样v51->v100就不需要被解析
+ SVN的增量链是全局的，若文件有100个版本，SVN必须从v100逐层计算差异到v1，时间复杂度为O(n)

#### 本地操作
+ Git：分布式。所有数据（包括完整历史和增量）存储在本地，解压操作完全本地化，速度极快
+ SVN：集中式。历史数据存储在远程服务器，回退时需要从服务器下载差异链，**网络延迟和带宽成为瓶颈**

## 数据清理
### filter-branch
filter-branch是官方提供的一个仓库清理工具，但其性能糟糕且存在大量安全隐患，因此不建议使用，[官方](https://git-scm.com/docs/git-filter-branch)也推荐使用其他的清理工具，如[filter-repo](https://github.com/newren/git-filter-repo/)

### filter-repo
[filter-repo](https://github.com/newren/git-filter-repo/)是一款多功能的重写历史工具，它大致属于与filter-branch相同的工具领域，但没有filter-branch那样令人望而却步的糟糕 性能，功能更多，并且其设计可扩展到超越简单重写情况的可用性。Git项目现在推荐使用filter-repo来代替filter-branch

虽然大多数用户可能只会将filter-repo用作简单的命令行工具（并且可能只使用其中的一些功能），但filter-repo的核心包含一个用于创建历史重写工具的库。因此，有特殊需求的用户可以利用它快速创建全新的历史重写工具

### BFG
BFG是一种更简单、更快捷的替代方法。相较于filter-branch，BFG在执行删除超大文件、密码、凭证、其他四人数据等方面更胜一筹。使用说明可参考[官方文档](https://rtyley.github.io/bfg-repo-cleaner/)，这里不再赘述，但是有两点需要注意

#### bare repo
再执行一系列命令时，需要使用裸仓库，可以使用如下命令克隆一个裸仓库
```
git clone --mirror git://example.com/some-big-repo.git
```

在执行克隆仓库时需要加上```--mirror```参数，此时克隆下来的仓库就不包含工作区，即只有```.git```目录下的内容
当然你也可以添加```--bare```参数
这两个参数唯一的区别是，使用```--mirror```克隆出来的裸仓库会对远程仓库进行注册，这样就可以使用fetch、pull、push等参数和远程仓库持续同步数据

#### protected commit
默认情况下，BFG不会修改分支上的最新/受保护提交的内容。这是因为最新提交很可能是部署到生产中的提交，而简单删除私人凭证或大文件很可能会导致代码损坏，不再具有它期望的硬编码数据。这时需要我们手动修复它，一旦提交了更改，并且最新提交是干净的，其中没有任何不需要的数据，我们就可以运行BFG对所有历史提交执行简单的删除操作

注意，虽然这些最新/受保护的提交中的文件不会被更改，但是当这些提交紧接着之前的脏提交时，它们的提交ID将会更改，以反映更改的历史记，只有文件系统树的哈希值会保持不变

当然如果你想关闭保护（一般不推荐），可以使用```--no-blob-protection```参数

#### 实操
```
~ ➜ ls
bfg.jar

# 克隆裸仓库
~ ➜ git clone --mirror git@github.com:debugxw/debugxw.github.io.git
克隆到纯仓库 'debugxw.github.io.git'...
remote: Enumerating objects: 689, done.
remote: Counting objects: 100% (689/689), done.
remote: Compressing objects: 100% (319/319), done.
remote: Total 689 (delta 165), reused 681 (delta 157), pack-reused 0 (from 0)
接收对象中: 100% (689/689), 12.55 MiB | 3.64 MiB/s, 完成.
处理 delta 中: 100% (165/165), 完成.

~ ➜ cd debugxw.github.io.git

~/debugxw.github.io.git ➜ git count-objects -vH
count: 0
size: 0 字节
in-pack: 689
packs: 1
size-pack: 12.57 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 字节

# 查找出占用空间最大的1个对象
# 这里可以看到563d4这个对象占用了1.9M的空间
~/debugxw.github.io.git ➜ git verify-pack -v objects/pack/pack-*.idx | grep -v chain | sort -k3nr | head -n 1
563d4d613144f8f7867de89545a9bc698eed3f7a blob   1978534 1941582 11069837

# 查看563d4所对应的文件名
~/debugxw.github.io.git ➜ git rev-list --objects --all | grep 563d4d613144f8f7867de89545a9bc698eed3f7a
563d4d613144f8f7867de89545a9bc698eed3f7a img/theme/026.jpg
~/debugxw.github.io.git ➜ git rev-list --objects --all | grep 026.jpg
563d4d613144f8f7867de89545a9bc698eed3f7a img/theme/026.jpg

~/debugxw.github.io.git ➜ cd ..

~ ➜ ls
bfg.jar               debugxw.github.io.git

~ ➜ java -jar bfg.jar --delete-files 026.jpg debugxw.github.io.git

Using repo :~/debugxw.github.io.git

Found 3 objects to protect
Found 6 commit-pointing refs : HEAD, refs/heads/deploy, refs/heads/master, ...
Found 1 tag-pointing refs : refs/tags/2025020701

Protected commits
-----------------

These are your protected commits, and so their contents will NOT be altered:

 * commit e8a9671b (protected by 'HEAD')

Cleaning
--------

Found 15 commits
Cleaning commits:       100% (15/15)
Cleaning commits completed in 46 ms.

Updating 1 Ref
--------------

	Ref                 Before     After
	---------------------------------------
	refs/heads/deploy | 2646a060 | bf9c44d5

Updating references:    100% (1/1)
...Ref update completed in 4 ms.

Commit Tree-Dirt History
------------------------

	Earliest      Latest
	|                  |
	.. ..D D.. ..D D.. m

	D = dirty commits (file tree fixed)
	m = modified commits (commit message or parents changed)
	. = clean commits (no changes to file tree)

	                        Before     After
	-------------------------------------------
	First modified commit | cf4c23bf | ec1edcf2
	Last dirty commit     | 2134fc12 | 1b307b75

Deleted files
-------------

	Filename   Git id
	----------------------------
	026.jpg  | 563d4d61 (1.9 MB)


In total, 14 object ids were changed. Full details are logged here:

	/Users/edy/xw/gittest1/debugxw.github.io.git.bfg-report/2025-02-07/16-54-01

BFG run is complete! When ready, run: git reflog expire --expire=now --all && git gc --prune=now --aggressive

➜ ~ cd debugxw.github.io.git

# BFG 将更新提交以及所有分支和标签，但它不会物理删除不需要的内容
# 当你确保历史记录已更新后，可以使用标准git gc命令删除不需要的脏数据
~/debugxw.github.io.git ➜ git reflog expire --expire=now --all && git gc --prune=now --aggressive
枚举对象中: 685, 完成.
对象计数中: 100% (685/685), 完成.
使用 8 个线程进行压缩
压缩对象中: 100% (472/472), 完成.
写入对象中: 100% (685/685), 完成.
Building bitmaps: 100% (15/15), 完成.
总共 685（差异 170），复用 497（差异 0），包复用 0（来自  0 个包）

# 再次查看磁盘空间占用，发现确实少了
~/debugxw.github.io.git ➜ git count-objects -vH
count: 0
size: 0 字节
in-pack: 685
packs: 1
size-pack: 10.66 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 字节

# 推送至远程仓库
~/debugxw.github.io.git ➜ git push --mirror
枚举对象中: 623, 完成.
写入对象中: 100% (623/623), 10.28 MiB | 3.03 MiB/s, 完成.
总共 623（差异 0），复用 0（差异 0），包复用 623（来自  1 个包）
remote: Resolving deltas: 100% (161/161), done.
To github.com:debugxw/debugxw.github.io.git
 + 2646a06...bf9c44d deploy -> deploy (forced update)
```

> 参考
  [Git内部原理](https://git-scm.com/book/zh/v2/Git-%e5%86%85%e9%83%a8%e5%8e%9f%e7%90%86-%e5%ba%95%e5%b1%82%e5%91%bd%e4%bb%a4%e4%b8%8e%e4%b8%8a%e5%b1%82%e5%91%bd%e4%bb%a4)
  [git仓库清理--"保姆级"教程](https://juejin.cn/post/7024922528514572302#heading-1)


