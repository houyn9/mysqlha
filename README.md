# mysqlha
主主模式

xtrabackup容器内脚本，注意脚本内的mysql地址用的headless，修改为自己的headless即可。

```
command:
        - bash
        - "-c"
        - |		
		  set -ex
          hostname=`hostname`
		  sleep 100
          tmpuser=`mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e "select user from mysql.user where user='$hostname' limit  1;"`
          if [[ ! $tmpuser ]];then
            mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e "CREATE USER '$hostname'@'%' IDENTIFIED BY 'repl' ;"
            mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e "GRANT REPLICATION SLAVE ON *.* TO '$hostname'@'%' ;"
            mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD -e "FLUSH PRIVILEGES ;"
          fi
          # 确定binlog 克隆数据位置(如果binlog存在的话).
          cd /var/lib/mysql
          # 如果存在该文件，则该xrabackup是从现有的从节点克隆出来的。
          
		if [[ $hostname = mysqlha-1 ]];then
			binlog0=`mysql -h mysqlha-0.mysqlha -uroot -p$ROOTPASSWORD -e "show master status"| grep mysql|awk '{print $1}'`
			postion0=`mysql -h mysqlha-0.mysqlha -uroot -p$ROOTPASSWORD -e "show master status"| grep mysql|awk '{print $2}'`
			mysql -h 127.0.0.1 -uroot -p$ROOTPASSWORD <<EOF
			stop slave;
			reset slave;
			CHANGE MASTER TO master_log_file='$binlog0',
			master_log_pos=$postion0,
			MASTER_HOST='mysqlha-0.mysqlha',
			MASTER_USER='mysqlha-0',
			MASTER_PASSWORD='repl',
			MASTER_CONNECT_RETRY=10;
			START SLAVE;
EOF
			binlog1=`mysql -h mysqlha-1.mysqlha -uroot -p$ROOTPASSWORD -e "show master status"| grep mysql|awk '{print $1}'`
			postion1=`mysql -h mysqlha-1.mysqlha -uroot -p$ROOTPASSWORD -e "show master status"| grep mysql|awk '{print $2}'`
			mysql -h mysqlha-0.mysqlha -uroot -p$ROOTPASSWORD <<EOF
			stop slave;
			reset slave;
			CHANGE MASTER TO master_log_file='$binlog1',
			master_log_pos=$postion1,
			MASTER_HOST='mysqlha-1.mysqlha',
			MASTER_USER='mysqlha-1',
			MASTER_PASSWORD='repl',
			MASTER_CONNECT_RETRY=10;
			START SLAVE;
EOF
			echo "set master-slave successfully"
		fi
        
		  
          exec ncat --listen --keep-open --send-only --max-conns=1 3307 -c \
            "xtrabackup --backup --slave-info --stream=xbstream --host=127.0.0.1 --user=root"
```
