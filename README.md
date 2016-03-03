# cpustat - high(er) frequency stats sampling

Cpustat is a tool for Linux systems to measure performance. You can think of it like a
fancy sort of "top" that does different things. This project is motived by Brendan Gregg's
[USE Method](http://www.brendangregg.com/usemethod.html) and tries to expose CPU
utilization and saturation in a helpful way.

Most performance tools average CPU usage over a few seconds or even a minute. This can
create the illusion of excess capacity because brief spikes in resource usage are blended
in with less busy periods. Cpustat takes higher frequency samples of every process running
on the machine and then summarizes these samples at a lower frequency. For example, it can
measure every process every 200ms and summarize these samples every 5 seconds, including
min/max/average values for some metrics.

There are two ways of displaying this data, a pure text list of the summary interval
and a colorful scrolling dashboard of each sample.

Here are examples of both modes observing the same worklad:

![Text Mode](http://imgur.com/vu4LrBD.gif)

![Demo](http://i.imgur.com/sf2QhVB.gif)

## Displayed Values

In pure text mode, there are some system-wide summary metrics that come from /proc/stat:

Name | Description
------------ | -------------
usr | min/max/avg user mode run time as a percentage of a CPU
sys | min/max/avg system mode run time as a percentage of a CPU
nice | min/max/avg user mode low priority run time as a percentage of a CPU
idle | min/max/avg user mode run time as a percentage of a CPU
iowait | min/max/avg user mode run time as a percentage of a CPU
prun | min/max/avg count of processes in a runnable state
pblock | min/max/avg count of processes blocked on disk IO
pstart | number of processes/threads started in this summary interval



## Data Sources


