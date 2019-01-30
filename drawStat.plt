set terminal png truecolor size 1024, 768 font "Helvetica,10"
set datafile separator "\t"
set title "CPU and Memory usage(Listeners count)\nCore2Duo E7500 2.93GHz, 4 GB RAM"
set output "ue_stat01tmp.png"
set grid ytics y2tics xtics
set grid
set key left top

# total memory size, Kbytes
totalMemoryGB = 4
totalMemory = totalMemoryGB*1024*1024

set xdata time
set timefmt "%Y-%m-%d %H:%M:%S"
set format x "%d.%m.%Y\n%H:%M"

set style line 1 lw 2 lt 1 lc 3
set style line 2 lw 2 lt 1 lc 1
set style line 3 lw 2 lt 1 lc 2

# Memory
set ylabel "Listeners"
set yrange [0:20000]
set ytics nomirror

# CPU
set y2label 'Usage, %'
set y2range [0:100]
set y2tics 5 nomirror
set autoscale y
#set autoscale y2


plot "log/stat.log" using 1:2 title "Listeners" with lines linestyle 1 axes x1y1, \
    "log/stat.log" using 1:(($4/totalMemory)*100) title "Memory" with lines linestyle 2 axes x1y2 , \
    "log/stat.log" using 1:3 title "CPU" with lines linestyle 3 axes x1y2
    

