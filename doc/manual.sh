# 最简单的代理
gost -L="socks5://:1081" -F="74.121.149.207:20080"
gost -L=:20080

gost -L="socks5://:40080" -F="socks5://36.27.223.4:49001"


# 同时支持tcp和udp
gost -L="socks5://:1081?udp" -F="74.121.149.207:20080?udp"
gost -L="tcp+udp://:20080"




iptables -t nat -D PREROUTING -p tcp -m tcp --dport 20080 -j REDIRECT --to-ports 12345
iptables -t nat -D PREROUTING -p udp -m udp --dport 20080 -j REDIRECT --to-ports 12345

gost -L=tcp+udp://:20080 -F=direct




gost -L=tcp://:12345 -F=direct

gost -L=tcp+udp://:12345 -F=direct

iptables -t nat -A PREROUTING -p tcp --dport 20080 -j REDIRECT --to-ports 12345
iptables -t nat -A PREROUTING -p udp --dport 20080 -j REDIRECT --to-ports 12345

iptables -t mangle -A PREROUTING -p tcp --sport 20080 -m state --state ESTABLISHED,RELATED -j MARK --set-mark 1
iptables -t mangle -A PREROUTING -p udp --sport 20080 -m state --state ESTABLISHED,RELATED -j MARK --set-mark 1

iptables -t nat -A PREROUTING -m mark --mark 1 -j DNAT --to-destination 36.27.223.4:49001



gost -L="socks5://:40081" -F="relay+tls://:443"
gost -L="socks5://127.0.0.1:49001" \
     -F="socks5://47.96.174.61:49001" \
     -F="relay+tls://74.121.149.207:443"