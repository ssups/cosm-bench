#!/bin/bash  
NODE_NUM=$1  
echo "Starting log capture for node ${NODE_NUM}" >&2  

while IFS= read -r line; do      
    if echo "$line" | grep -q "committed state"; then  
        clean_line=$(echo "$line" | sed -r "s/\x1B\[[0-9;]*[mK]//g")  
        timestamp=$(perl -MTime::HiRes=time -e 'printf "%d",time()*1000')  
        echo "$timestamp $clean_line" >> "./logs/output${NODE_NUM}.log"  
    fi  
done  

echo "Log capture ended for node ${NODE_NUM}" >&2  