MIT 6.824 lab1

lab 1主要实现一个简单的mapReduce，它主要有四个部分

### Part I: Map/Reduce input and output

第一部分主要是实现doMap和doReduce两个部分。对于每一次调用，map端做这么几件事情：

1.  读取内容
2.  生成map result的临时文件
3.  将不同的key的结果存入不同的文件，存入哪里由`key%ihash`决定

doReduc做这样几件事情：

1.  读取map的结果
2.  对于key进行排序。（这里简单粗暴的做了内存排序，没往磁盘split）
3.  调用reduce并且将结果存入文件

### Part II: Single-worker word count

实现workcount的map和reduce函数。很简单

### Part III: Distributing MapReduce tasks

之前都是调用串行模式，这个part调用并行模式，所以要实现scheduler。框架提供的是用socket实现的rpc，是在本地模拟多线程。实现时通过channel组成一个空闲worker队列，每次用go并行的给worker分配任务。同时scheduler要加锁直到所有worker都执行完才推出。

### Part IV: Handling worker failures

这一部分只要part 3写对就没什么问题，就是在一个worker出错时重新分配这个任务到其他worker去就好。



lab1主要时间还是花在熟悉go语言上了，不过这东西写并行真是快。go和channel真好用。

lab主页<http://nil.csail.mit.edu/6.824/2016/labs/lab-1.html>

github代码<https://github.com/thousandhu/MIT6_824>
