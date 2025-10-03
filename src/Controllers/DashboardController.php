<?php
/**
 * 仪表板控制器
 */

namespace App\Controllers;

use App\Core\Database;

class DashboardController extends BaseController
{
    /**
     * 显示仪表板
     */
    public function index(): void
    {
        $this->requireAuth();

        $db = Database::getInstance();

        // 统计数据
        $stats = [
            'total_users' => $db->querySingle('SELECT COUNT(*) FROM proxy_users'),
            'active_users' => $db->querySingle('SELECT COUNT(*) FROM proxy_users WHERE enabled = 1 AND expiry_date >= date("now")'),
            'expired_users' => $db->querySingle('SELECT COUNT(*) FROM proxy_users WHERE expiry_date < date("now")'),
            'total_nodes' => $db->querySingle('SELECT COUNT(*) FROM inbound_nodes'),
            'active_nodes' => $db->querySingle('SELECT COUNT(*) FROM inbound_nodes WHERE enabled = 1'),
            'total_servers' => $db->querySingle('SELECT COUNT(*) FROM servers'),
            'active_servers' => $db->querySingle('SELECT COUNT(*) FROM servers WHERE enabled = 1'),
        ];

        // 最近部署日志
        $recentDeploys = [];
        $result = $db->query('
            SELECT dl.*, s.name as server_name 
            FROM deploy_logs dl
            LEFT JOIN servers s ON s.id = dl.server_id
            ORDER BY dl.created_at DESC
            LIMIT 10
        ');
        
        while ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            $recentDeploys[] = $row;
        }

        // 即将过期的用户
        $expiringUsers = [];
        $result = $db->query('
            SELECT * FROM proxy_users 
            WHERE expiry_date BETWEEN date("now") AND date("now", "+7 days")
            AND enabled = 1
            ORDER BY expiry_date ASC
            LIMIT 10
        ');
        
        while ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            $expiringUsers[] = $row;
        }

        $this->view('dashboard/index', [
            'username' => $this->getUsername(),
            'stats' => $stats,
            'recent_deploys' => $recentDeploys,
            'expiring_users' => $expiringUsers,
        ]);
    }
}
