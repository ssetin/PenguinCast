set terminal png size 1024, 768 font "Helvetica,10"
set datafile separator "\t"
set title "CPU and Memory usage(Listeners count)"
set output "Stat01.png"
set grid ytics y2tics xtics
set grid

set xdata time
set timefmt "%Y-%m-%d %H:%M:%S"
set format x "%d-%b\n%H:%M"

set style line 1 lw 2 lt 1 lc 3
set style line 2 lw 2 lt 1 lc 1
set style line 3 lw 2 lt 1 lc 2

# Memory
set ylabel "[Listeners], [Memory, MB]"
set yrange [0:4096]
set ytics nomirror

# CPU
set y2label '[CPU, %]'
set y2range [0:100]
set format y2 '%.0s%%'
set y2tics 10 nomirror
set autoscale y


plot "log/stat.log" using 1:2 title "Listeners" with lines linestyle 1 axes x1y1, \
    "log/stat.log" using 1:($4/1024) title "Mem" with lines linestyle 2 axes x1y1 , \
    "log/stat.log" using 1:3 title "CPU" with lines linestyle 3 axes x1y2
    

