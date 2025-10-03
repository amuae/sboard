<?php
/**
 * 订阅链接生成器
 * 根据用户UUID、入站节点类型、用户等级生成订阅配置
 */

require_once __DIR__ . '/../src/autoload.php';

use App\Core\Database;

// 获取参数
$ua = $_GET['ua'] ?? detectUserAgent();
$userUuid = $_GET['user'] ?? '';
$nodeType = $_GET['type'] ?? '';
$lv = isset($_GET['lv']) ? (int)$_GET['lv'] : null;

// 参数验证
if (empty($userUuid)) {
    http_response_code(400);
    die('错误: 缺少用户UUID参数');
}

if (empty($nodeType)) {
    http_response_code(400);
    die('错误: 缺少节点类型参数');
}

// 获取数据库实例
$db = Database::getInstance();

// 查询用户信息
$stmt = $db->prepare('SELECT * FROM proxy_users WHERE uuid = ?');
$stmt->bindValue(1, $userUuid, SQLITE3_TEXT);
$result = $stmt->execute();
$user = $result->fetchArray(SQLITE3_ASSOC);

if (!$user) {
    http_response_code(404);
    die('错误: 用户不存在');
}

// 检查用户是否启用
if (!$user['enabled']) {
    http_response_code(403);
    die('错误: 用户已被禁用');
}

// 检查用户是否过期
if (strtotime($user['expiry_date']) < time()) {
    http_response_code(403);
    die('错误: 用户已过期');
}

// 获取用户等级
$userLevel = (int)$user['level'];

// 查询入站节点
$stmt = $db->prepare('SELECT * FROM inbound_nodes WHERE tag = ? AND enabled = 1');
$stmt->bindValue(1, $nodeType, SQLITE3_TEXT);
$result = $stmt->execute();
$inboundNode = $result->fetchArray(SQLITE3_ASSOC);

if (!$inboundNode) {
    http_response_code(404);
    die('错误: 入站节点不存在或已禁用');
}

// 确定服务器分类过滤条件
$categoryFilter = '';
if ($userLevel == 3 && $lv === 3) {
    // 3级用户且指定lv=3：只获取家宽服务器
    $categoryFilter = "AND category = 'home'";
} elseif ($userLevel == 3 && $lv === 2) {
    // 3级用户且指定lv=2：只获取直连和中转服务器
    $categoryFilter = "AND category IN ('direct', 'relay')";
} elseif ($userLevel == 3) {
    // 3级用户未指定lv：获取所有服务器
    $categoryFilter = '';
} else {
    // 1级和2级用户：只能获取直连和中转服务器
    $categoryFilter = "AND category IN ('direct', 'relay')";
}

// 查询符合条件的服务器
$query = "SELECT * FROM servers WHERE enabled = 1 {$categoryFilter} ORDER BY id";
$servers = [];
$result = $db->query($query);
while ($row = $result->fetchArray(SQLITE3_ASSOC)) {
    $servers[] = $row;
}

if (empty($servers)) {
    http_response_code(404);
    die('错误: 没有可用的服务器');
}

// 根据UA生成对应格式的订阅
switch (strtolower($ua)) {
    case 'mihomo':
    case 'clash':
        generateMihomoSubscription($user, $inboundNode, $servers, $userLevel, $lv);
        break;
    case 'sing-box':
        generateSingBoxSubscription($user, $inboundNode, $servers, $userLevel, $lv);
        break;
    case 'shadowrocket':
    case 'v2ray':
    default:
        generateV2raySubscription($user, $inboundNode, $servers, $userLevel, $lv);
        break;
}

/**
 * 检测User-Agent
 */
function detectUserAgent() {
    $userAgent = $_SERVER['HTTP_USER_AGENT'] ?? '';

    if (stripos($userAgent, 'sing-box') !== false) {
        return 'sing-box';
    } elseif (stripos($userAgent, 'clash') !== false || stripos($userAgent, 'mihomo') !== false) {
        return 'mihomo';
    } elseif (stripos($userAgent, 'shadowrocket') !== false) {
        return 'shadowrocket';
    } elseif (stripos($userAgent, 'v2ray') !== false || stripos($userAgent, 'v2rayng') !== false) {
        return 'v2ray';
    }

    // 浏览器默认返回mihomo
    return 'mihomo';
}

/**
 * 获取节点名称
 */
function getNodeName($server, $userLevel, $lv) {
    // 如果是3级用户且指定了lv参数
    if ($userLevel == 3 && $lv !== null) {
        if ($lv == 3) {
            // lv=3使用node_3
            return $server['node_3'] ?: $server['name'];
        } elseif ($lv == 2) {
            // lv=2使用node_2
            return $server['node_2'] ?: $server['name'];
        }
    }

    // 1级用户使用node_1
    if ($userLevel == 1) {
        return $server['node_1'] ?: $server['name'];
    }

    // 2级和3级用户(无lv参数)使用node_2
    if ($userLevel == 2 || $userLevel == 3) {
        return $server['node_2'] ?: $server['name'];
    }

    return $server['name'];
}

/**
 * 应用DNS解析策略
 */
function applyDnsResolve($host, $dnsResolve) {
    // 标准化 DNS 解析类型
    $dnsResolve = strtolower(trim($dnsResolve));

    // none 或空值表示不解析
    if ($dnsResolve === 'none' || $dnsResolve === '' || $dnsResolve === '0' || $dnsResolve == 0) {
        return $host;
    }

    // 如果已经是 IP 地址，直接返回
    if (filter_var($host, FILTER_VALIDATE_IP)) {
        return $host;
    }

    // 解析为 IPv4
    if ($dnsResolve === 'ipv4' || $dnsResolve === '4' || $dnsResolve == 4) {
        $records = @dns_get_record($host, DNS_A);
        if (!empty($records) && isset($records[0]['ip'])) {
            return $records[0]['ip'];
        }
    }
    // 解析为 IPv6
    elseif ($dnsResolve === 'ipv6' || $dnsResolve === '6' || $dnsResolve == 6) {
        $records = @dns_get_record($host, DNS_AAAA);
        if (!empty($records) && isset($records[0]['ipv6'])) {
            return $records[0]['ipv6'];
        }
    }

    // 解析失败，返回原主机名
    return $host;
}

/**
 * 生成Mihomo/Clash订阅 (严格按照官方文档)
 */
function generateMihomoSubscription($user, $inboundNode, $servers, $userLevel, $lv) {
    $proxies = [];
    $proxyNames = [];

    foreach ($servers as $server) {
        $nodeName = getNodeName($server, $userLevel, $lv);
        $serverHost = applyDnsResolve($server['host'], $server['dns_resolve']);

        $proxy = [
            'name' => $nodeName,
            'type' => $inboundNode['protocol'],
            'server' => $serverHost,
            'port' => (int)$inboundNode['port'],
            'udp' => true  // 官方文档:所有协议都应有udp字段
        ];

        // 协议特定配置 (严格按照官方wiki)
        if ($inboundNode['protocol'] === 'trojan') {
            // Trojan: password (必须)
            $proxy['password'] = $user['uuid'];
        } elseif ($inboundNode['protocol'] === 'vless') {
            // VLESS: uuid (必须), flow (可选), packet-encoding (可选)
            $proxy['uuid'] = $user['uuid'];
            if ($inboundNode['flow']) {
                $proxy['flow'] = $inboundNode['flow'];  // 如: xtls-rprx-vision
            }
            // packet-encoding: xudp (xray支持) / packetaddr (v2ray 5+支持)
            $proxy['packet-encoding'] = 'xudp';
        } elseif ($inboundNode['protocol'] === 'vmess') {
            // VMess: uuid (必须), alterId (必须), cipher (必须)
            $proxy['uuid'] = $user['uuid'];
            $proxy['alterId'] = 0;  // 0=新协议, 非0=旧协议
            $proxy['cipher'] = 'auto';  // auto/none/zero/aes-128-gcm/chacha20-poly1305
            // packet-encoding: xudp (xray支持) / packetaddr (v2ray 5+支持)
            $proxy['packet-encoding'] = 'xudp';
        } elseif ($inboundNode['protocol'] === 'anytls') {
            // AnyTLS: password (必须), 会话管理参数
            $proxy['password'] = $user['uuid'];
            $proxy['idle-session-check-interval'] = 30;  // 检查空闲会话的时间间隔(秒)
            $proxy['idle-session-timeout'] = 30;  // 关闭闲置会话的超时时间(秒)
            $proxy['min-idle-session'] = 0;  // 至少保持打开的空闲会话数
        }

        // TLS配置 (严格按照官方文档)
        if ($inboundNode['tls_enabled']) {
            $proxy['tls'] = true;

            // Trojan 和 AnyTLS 使用 sni，其他协议使用 servername
            if ($inboundNode['protocol'] === 'trojan' || $inboundNode['protocol'] === 'anytls') {
                $proxy['sni'] = $inboundNode['server_name'];
            } else {
                $proxy['servername'] = $inboundNode['server_name'];
            }

            // 添加 ALPN 支持（应用层协议协商）
            $proxy['alpn'] = ['h2', 'http/1.1'];

            // 添加客户端指纹模拟（提高连接成功率）
            $proxy['client-fingerprint'] = 'chrome';

            $proxy['skip-cert-verify'] = true;

            if ($inboundNode['reality_enabled']) {
                // Reality配置
                $proxy['reality-opts'] = [
                    'public-key' => $inboundNode['reality_pubkey'],
                    'short-id' => $inboundNode['reality_short_id'] ?? ''
                ];
                // Reality必须配合flow使用(VLESS)
                if ($inboundNode['protocol'] === 'vless' && $inboundNode['flow']) {
                    $proxy['flow'] = $inboundNode['flow'];
                }
            }
        }

        // 传输层配置
        if ($inboundNode['transport_enabled']) {
            $proxy['network'] = $inboundNode['transport_type'];

            if ($inboundNode['transport_type'] === 'ws') {
                $proxy['ws-opts'] = [
                    'path' => $inboundNode['ws_path'] ?: '/'
                ];
            } elseif ($inboundNode['transport_type'] === 'grpc') {
                $proxy['grpc-opts'] = [
                    'grpc-service-name' => $inboundNode['grpc_service']
                ];
            } elseif ($inboundNode['transport_type'] === 'http') {
                $proxy['http-opts'] = [
                    'path' => [$inboundNode['ws_path'] ?: '/']
                ];
            }
        }

        $proxies[] = $proxy;
        $proxyNames[] = $nodeName;
    }

    $config = [
        'mixed-port' => 7890,
        'allow-lan' => false,
        'mode' => 'rule',
        'log-level' => 'info',
        'external-controller' => '127.0.0.1:9090',
        'proxies' => $proxies,
        'proxy-groups' => [
            [
                'name' => '自动选择',
                'type' => 'url-test',
                'proxies' => $proxyNames,
                'url' => 'https://www.gstatic.com/generate_204',
                'interval' => 300
            ],
            [
                'name' => '手动选择',
                'type' => 'select',
                'proxies' => array_merge(['自动选择'], $proxyNames)
            ]
        ],
        'rules' => [
            'DOMAIN-SUFFIX,local,DIRECT',
            'IP-CIDR,127.0.0.0/8,DIRECT',
            'IP-CIDR,192.168.0.0/16,DIRECT',
            'IP-CIDR,10.0.0.0/8,DIRECT',
            'IP-CIDR,172.16.0.0/12,DIRECT',
            'GEOIP,CN,DIRECT',
            'MATCH,手动选择'
        ]
    ];

    header('Content-Type: text/yaml; charset=utf-8');
    header('Content-Disposition: inline');
    header('Subscription-Userinfo: upload=0; download=' . ($user['traffic_used'] * 1073741824) . '; total=' . ($user['traffic_limit'] * 1073741824) . '; expire=' . strtotime($user['expiry_date']));
    header('Profile-Update-Interval: 24');
    echo arrayToYaml($config);
    exit;
}

function generateSingBoxSubscription($user, $inboundNode, $servers, $userLevel, $lv)
{
    $outbounds = [];
    $tags = [];

    foreach ($servers as $server) {
        $nodeName = getNodeName($server, $userLevel, $lv);
        $serverHost = applyDnsResolve($server['host'], $server['dns_resolve']);

        $outbound = [
            'type' => $inboundNode['protocol'],
            'tag' => $nodeName,
            'server' => $serverHost,
            'server_port' => (int)$inboundNode['port'],
        ];

        /* ---------- 协议层 ---------- */
        if ($inboundNode['protocol'] === 'trojan') {
            $outbound['password'] = $user['uuid'];
            $outbound['network']  = 'tcp';
        } elseif ($inboundNode['protocol'] === 'vless') {
            $outbound['uuid']            = $user['uuid'];
            $outbound['flow']            = $inboundNode['flow'] ?: 'xtls-rprx-vision';
            $outbound['network']         = 'tcp';
            $outbound['packet_encoding'] = 'xudp';   // 必填，支持 UDP
        } elseif ($inboundNode['protocol'] === 'vmess') {
            $outbound['uuid']            = $user['uuid'];
            $outbound['alter_id']        = 0;
            $outbound['security']        = 'auto';
            $outbound['network']         = 'tcp';
            $outbound['packet_encoding'] = 'xudp';   // 必填，支持 UDP
        } elseif ($inboundNode['protocol'] === 'anytls') {
            $outbound['password']                   = $user['uuid'];
            $outbound['idle_session_check_interval'] = '30s';
            $outbound['idle_session_timeout']        = '30s';
            $outbound['min_idle_session']            = 0;
        }

        /* ---------- TLS / Reality ---------- */
        if ($inboundNode['tls_enabled']) {
            if ($inboundNode['reality_enabled']) {
                // Reality 模式：去掉 insecure，否则 1.12+ 报错
                $outbound['tls'] = [
                    'enabled'     => true,
                    'server_name' => $inboundNode['server_name'],
                    'utls'        => [
                        'enabled'     => true,
                        'fingerprint' => 'chrome',
                    ],
                    'reality' => [
                        'enabled'    => true,
                        'public_key' => $inboundNode['reality_pubkey'],
                        'short_id'   => $inboundNode['reality_short_id'] ?? '',
                    ],
                ];
            } else {
                // 普通 TLS
                $outbound['tls'] = [
                    'enabled'     => true,
                    'server_name' => $inboundNode['server_name'],
                    'insecure'    => true,
                ];
            }
        }

        /* ---------- 传输层 ---------- */
        if ($inboundNode['transport_enabled']) {
            if ($inboundNode['transport_type'] === 'ws') {
                $outbound['transport'] = [
                    'type'                     => 'ws',
                    'path'                     => $inboundNode['ws_path'] ?: '/',
                    'max_early_data'           => 0,
                    'early_data_header_name'   => 'Sec-WebSocket-Protocol',
                ];
                if (!empty($inboundNode['ws_headers'])) {
                    $outbound['transport']['headers'] = json_decode($inboundNode['ws_headers'], true) ?: [];
                }
            } elseif ($inboundNode['transport_type'] === 'grpc') {
                $outbound['transport'] = [
                    'type'         => 'grpc',
                    'service_name' => $inboundNode['grpc_service'],
                ];
            } elseif ($inboundNode['transport_type'] === 'http') {
                $outbound['transport'] = [
                    'type'   => 'http',
                    'path'   => $inboundNode['ws_path'] ?: '/',
                    'method' => 'GET',
                ];
                if (!empty($inboundNode['server_name'])) {
                    $outbound['transport']['host'] = [$inboundNode['server_name']];
                }
            }
        }

        $outbounds[] = $outbound;
        $tags[]      = $nodeName;
    }

    /* ---------- 自动 / 手动 选择 ---------- */
    $outbounds[] = [
        'type'      => 'urltest',
        'tag'       => '自动选择',
        'outbounds' => $tags,
        'url'       => 'https://www.gstatic.com/generate_204',
        'interval'  => '5m',
        'tolerance' => 50,
    ];
    $outbounds[] = [
        'type'     => 'selector',
        'tag'      => '手动选择',
        'outbounds' => array_merge(['自动选择'], $tags),
        'default'  => '自动选择',
    ];

    /* ---------- 基础出站 ---------- */
    $outbounds[] = ['type' => 'direct', 'tag' => 'direct'];
    $outbounds[] = ['type' => 'dns',    'tag' => 'dns-out'];
    $outbounds[] = ['type' => 'block',  'tag' => 'block'];

    /* ---------- 完整配置 ---------- */
    $config = [
        'log' => [
            'level'      => 'info',
            'timestamp'  => true,
        ],
        'dns' => [
            'servers' => [
                [
                    'type'        => 'https',
                    'tag'         => 'google',
                    'server'      => '8.8.8.8',
                    'server_port' => 443,
                    'path'        => '/dns-query',
                    'detour'      => '手动选择',
                ],
                [
                    'type'        => 'udp',
                    'tag'         => 'local',
                    'server'      => '223.5.5.5',
                    'server_port' => 53,
                    'detour'      => 'direct',
                ],
            ],
            'rules' => [
                [
                    'rule_set' => 'geosite-cn',
                    'server'   => 'local',
                ],
            ],
            'final'    => 'google',
            'strategy' => 'prefer_ipv4',
        ],
        'inbounds' => [
            [
                'type'                   => 'mixed',
                'tag'                    => 'mixed-in',
                'listen'                 => '127.0.0.1',
                'listen_port'            => 2080,
                'sniff'                  => true,
                'sniff_override_destination' => false,
            ],
        ],
        'outbounds' => $outbounds,
        'route' => [
            'rules' => [
                [
                    'protocol' => 'dns',
                    'outbound' => 'dns-out',
                ],
                [
                    'rule_set' => 'geosite-cn',
                    'outbound' => 'direct',
                ],
                [
                    'rule_set' => 'geoip-cn',
                    'outbound' => 'direct',
                ],
                [
                    'ip_is_private' => true,
                    'outbound'      => 'direct',
                ],
            ],
            'rule_set' => [
                [
                    'type'   => 'remote',
                    'tag'    => 'geosite-cn',
                    'format' => 'binary',
                    'url'    => 'https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs',
                ],
                [
                    'type'   => 'remote',
                    'tag'    => 'geoip-cn',
                    'format' => 'binary',
                    'url'    => 'https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs',
                ],
            ],
            'final'               => '手动选择',
            'auto_detect_interface' => true,
        ],
        'experimental' => [
            'cache_file' => [
                'enabled' => true,
                'path'    => 'cache.db',
            ],
        ],
    ];

    header('Content-Type: application/json; charset=utf-8');
    header('Content-Disposition: inline');
    header('Subscription-Userinfo: upload=0; download=' . ($user['traffic_used'] * 1073741824) . '; total=' . ($user['traffic_limit'] * 1073741824) . '; expire=' . strtotime($user['expiry_date']));
    header('Profile-Update-Interval: 24');
    echo json_encode($config, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
    exit;
}

/**
 * 生成V2Ray/Shadowrocket订阅
 * 注意：本系统将服务器域名解析为IP地址，因此必须跳过证书验证
 * 这是因为TLS证书是针对域名签发的，无法验证IP地址
 */
function generateV2raySubscription($user, $inboundNode, $servers, $userLevel, $lv) {
    $links = [];

    foreach ($servers as $server) {
        $nodeName = getNodeName($server, $userLevel, $lv);
        $serverHost = applyDnsResolve($server['host'], $server['dns_resolve']); // 域名解析为IP

        $protocol = $inboundNode['protocol'];
        $port = $inboundNode['port'];

        if ($protocol === 'trojan') {
            // Trojan链接格式
            $params = [];
            if ($inboundNode['tls_enabled']) {
                $params[] = 'sni=' . urlencode($inboundNode['server_name']);
                // 跳过证书验证 - 必需项：服务器域名解析为IP时证书验证会失败
                $params[] = 'allowInsecure=1';
                $params[] = 'skip-cert-verify=true';
            }
            if ($inboundNode['transport_enabled']) {
                $params[] = 'type=' . $inboundNode['transport_type'];
                if ($inboundNode['transport_type'] === 'ws') {
                    $params[] = 'path=' . urlencode($inboundNode['ws_path'] ?: '/');
                } elseif ($inboundNode['transport_type'] === 'grpc') {
                    $params[] = 'serviceName=' . urlencode($inboundNode['grpc_service']);
                }
            }

            $link = sprintf(
                'trojan://%s@%s:%d?%s#%s',
                $user['uuid'],
                $serverHost,
                $port,
                implode('&', $params),
                            urlencode($nodeName)
            );
            $links[] = $link;

        } elseif ($protocol === 'vless') {
            // VLESS链接格式
            $params = [];
            $params[] = 'encryption=none';

            if ($inboundNode['tls_enabled']) {
                if ($inboundNode['reality_enabled']) {
                    $params[] = 'security=reality';
                    $params[] = 'sni=' . urlencode($inboundNode['server_name']);
                    $params[] = 'pbk=' . urlencode($inboundNode['reality_pubkey']);
                    $params[] = 'sid=' . urlencode($inboundNode['reality_short_id'] ?? '');
                    if ($inboundNode['flow']) {
                        $params[] = 'flow=' . urlencode($inboundNode['flow']);
                    }
                } else {
                    $params[] = 'security=tls';
                    $params[] = 'sni=' . urlencode($inboundNode['server_name']);
                    // 跳过证书验证 - 必需项：服务器域名解析为IP时证书验证会失败
                    $params[] = 'allowInsecure=1';
                    $params[] = 'skip-cert-verify=true';
                }
            } else {
                $params[] = 'security=none';
            }

            if ($inboundNode['transport_enabled']) {
                $params[] = 'type=' . $inboundNode['transport_type'];
                if ($inboundNode['transport_type'] === 'ws') {
                    $params[] = 'path=' . urlencode($inboundNode['ws_path'] ?: '/');
                } elseif ($inboundNode['transport_type'] === 'grpc') {
                    $params[] = 'serviceName=' . urlencode($inboundNode['grpc_service']);
                }
            } else {
                $params[] = 'type=tcp';
            }

            $link = sprintf(
                'vless://%s@%s:%d?%s#%s',
                $user['uuid'],
                $serverHost,
                $port,
                implode('&', $params),
                            urlencode($nodeName)
            );
            $links[] = $link;

        } elseif ($protocol === 'vmess') {
            // VMess链接格式
            $vmessConfig = [
                'v' => '2',
                'ps' => $nodeName,
                'add' => $serverHost,
                'port' => (string)$port,
                'id' => $user['uuid'],
                'aid' => '0',
                'scy' => 'auto',
                'net' => $inboundNode['transport_enabled'] ? $inboundNode['transport_type'] : 'tcp',
                'type' => 'none',
                'host' => '',
                'path' => '',
                'tls' => $inboundNode['tls_enabled'] ? 'tls' : '',
                'sni' => $inboundNode['tls_enabled'] ? $inboundNode['server_name'] : '',
                // 跳过证书验证 - 必需项：服务器域名解析为IP时证书验证会失败
                'skip-cert-verify' => $inboundNode['tls_enabled'] ? true : false,
                'allowInsecure' => $inboundNode['tls_enabled'] ? 1 : 0
            ];

            if ($inboundNode['transport_enabled']) {
                if ($inboundNode['transport_type'] === 'ws') {
                    $vmessConfig['path'] = $inboundNode['ws_path'] ?: '/';
                } elseif ($inboundNode['transport_type'] === 'grpc') {
                    $vmessConfig['path'] = $inboundNode['grpc_service'];
                }
            }

            $link = 'vmess://' . base64_encode(json_encode($vmessConfig));
            $links[] = $link;

        } elseif ($protocol === 'anytls') {
            // AnyTLS链接格式 (Shadowrocket/Mihomo支持)
            $params = [];
            $params[] = 'udp=1';
            $params[] = 'idle-session-check-interval=30';
            $params[] = 'idle-session-timeout=30';
            $params[] = 'min-idle-session=0';

            if ($inboundNode['tls_enabled']) {
                $params[] = 'sni=' . urlencode($inboundNode['server_name']);
                // 跳过证书验证 - 必需项：服务器域名解析为IP时证书验证会失败
                $params[] = 'skip-cert-verify=1';
                $params[] = 'allowInsecure=1';
            }

            $link = sprintf(
                'anytls://%s@%s:%d?%s#%s',
                $user['uuid'],
                $serverHost,
                $port,
                implode('&', $params),
                            urlencode($nodeName)
            );
            $links[] = $link;
        }
    }

    header('Content-Type: text/plain; charset=utf-8');
    header('Subscription-Userinfo: upload=0; download=' . ($user['traffic_used'] * 1073741824) . '; total=' . ($user['traffic_limit'] * 1073741824) . '; expire=' . strtotime($user['expiry_date']));
    header('Profile-Update-Interval: 24');
    echo base64_encode(implode("\n", $links));
    exit;
}

/**
 * 数组转YAML
 */
function arrayToYaml($array, $indent = 0) {
    $yaml = '';
    foreach ($array as $key => $value) {
        if (is_array($value)) {
            if (isset($value[0])) {
                // 索引数组
                $yaml .= str_repeat('  ', $indent) . $key . ":\n";
                foreach ($value as $item) {
                    if (is_array($item)) {
                        // 关联数组项
                        $first = true;
                        foreach ($item as $subKey => $subValue) {
                            if ($first) {
                                if (is_array($subValue)) {
                                    // 嵌套数组
                                    $yaml .= str_repeat('  ', $indent + 1) . "- " . $subKey . ":\n";
                                    if (isset($subValue[0])) {
                                        // 索引数组
                                        foreach ($subValue as $subItem) {
                                            $yaml .= str_repeat('  ', $indent + 3) . "- " . formatYamlValue($subItem) . "\n";
                                        }
                                    } else {
                                        $yaml .= arrayToYaml($subValue, $indent + 3);
                                    }
                                } else {
                                    $yaml .= str_repeat('  ', $indent + 1) . "- " . $subKey . ": " . formatYamlValue($subValue) . "\n";
                                }
                                $first = false;
                            } else {
                                if (is_array($subValue)) {
                                    // 嵌套数组
                                    $yaml .= str_repeat('  ', $indent + 2) . $subKey . ":\n";
                                    if (isset($subValue[0])) {
                                        // 索引数组
                                        foreach ($subValue as $subItem) {
                                            $yaml .= str_repeat('  ', $indent + 3) . "- " . formatYamlValue($subItem) . "\n";
                                        }
                                    } else {
                                        $yaml .= arrayToYaml($subValue, $indent + 3);
                                    }
                                } else {
                                    $yaml .= str_repeat('  ', $indent + 2) . $subKey . ": " . formatYamlValue($subValue) . "\n";
                                }
                            }
                        }
                    } else {
                        // 简单值
                        $yaml .= str_repeat('  ', $indent + 1) . "- " . formatYamlValue($item) . "\n";
                    }
                }
            } else {
                // 关联数组
                $yaml .= str_repeat('  ', $indent) . $key . ":\n";
                $yaml .= arrayToYaml($value, $indent + 1);
            }
        } else {
            $yaml .= str_repeat('  ', $indent) . $key . ": " . formatYamlValue($value) . "\n";
        }
    }
    return $yaml;
}

/**
 * 格式化YAML值
 */
function formatYamlValue($value) {
    if (is_bool($value)) {
        return $value ? 'true' : 'false';
    } elseif (is_numeric($value)) {
        return $value;
    } elseif (is_string($value) && (strpos($value, ':') !== false || strpos($value, '#') !== false)) {
        return "'" . $value . "'";
    }
    return $value;
}
