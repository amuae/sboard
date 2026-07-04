<template>
  <div class="container-fluid mt-4">
    <div class="d-flex justify-content-between mb-4">
      <h2>订阅配置</h2>
    </div>

    <!-- Mihomo 配置区域 -->
    <div class="card mb-4">
      <div class="card-header d-flex justify-content-between align-items-center">
        <h5 class="mb-0">
          <i class="bi bi-box me-2"></i>Mihomo 配置
        </h5>
        <button class="btn btn-primary btn-sm" @click="addMihomoConfig">
          <i class="bi bi-plus-lg me-1"></i>新增配置
        </button>
      </div>
      <div class="card-body">
        <!-- 配置列表 -->
        <div v-if="mihomoConfigs.length === 0" class="text-center text-muted py-5">
          <i class="bi bi-inbox display-4 d-block mb-3"></i>
          <p>暂无 Mihomo 配置，点击上方按钮创建</p>
        </div>
        <div v-else class="row g-3">
          <div v-for="config in mihomoConfigs" :key="config.id" class="col-md-6 col-lg-4">
            <div class="card h-100" :class="{ 'border-primary': config.enabled }">
              <div class="card-header d-flex justify-content-between align-items-center py-2">
                <div class="d-flex align-items-center">
                  <div class="form-check form-switch me-2">
                    <input class="form-check-input" type="checkbox" 
                           :id="'mihomo-enable-' + config.id"
                           v-model="config.enabled"
                           @change="onMihomoToggle(config)">
                  </div>
                  <strong>{{ config.name }}</strong>
                </div>
                <div class="btn-group btn-group-sm">
                  <button class="btn btn-outline-primary" @click="editMihomoConfig(config)" title="编辑">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-outline-danger" @click="deleteMihomoConfig(config)" title="删除">
                    <i class="bi bi-trash"></i>
                  </button>
                </div>
              </div>
              <div class="card-body py-2">
                <small class="text-muted">{{ config.description || '暂无描述' }}</small>
                <div class="mt-2">
                  <span v-for="module in config.modules" :key="module" 
                        class="badge bg-secondary me-1 mb-1">{{ module }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox 配置区域 -->
    <div class="card mb-4">
      <div class="card-header d-flex justify-content-between align-items-center">
        <h5 class="mb-0">
          <i class="bi bi-box-seam me-2"></i>SingBox 配置
        </h5>
        <button class="btn btn-primary btn-sm" @click="addSingBoxConfig">
          <i class="bi bi-plus-lg me-1"></i>新增配置
        </button>
      </div>
      <div class="card-body">
        <!-- 配置列表 -->
        <div v-if="singboxConfigs.length === 0" class="text-center text-muted py-5">
          <i class="bi bi-inbox display-4 d-block mb-3"></i>
          <p>暂无 SingBox 配置，点击上方按钮创建</p>
        </div>
        <div v-else class="row g-3">
          <div v-for="config in singboxConfigs" :key="config.id" class="col-md-6 col-lg-4">
            <div class="card h-100" :class="{ 'border-primary': config.enabled }">
              <div class="card-header d-flex justify-content-between align-items-center py-2">
                <div class="d-flex align-items-center">
                  <div class="form-check form-switch me-2">
                    <input class="form-check-input" type="checkbox" 
                           :id="'singbox-enable-' + config.id"
                           v-model="config.enabled"
                           @change="onSingBoxToggle(config)">
                  </div>
                  <strong>{{ config.name }}</strong>
                </div>
                <div class="btn-group btn-group-sm">
                  <button class="btn btn-outline-primary" @click="editSingBoxConfig(config)" title="编辑">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-outline-danger" @click="deleteSingBoxConfig(config)" title="删除">
                    <i class="bi bi-trash"></i>
                  </button>
                </div>
              </div>
              <div class="card-body py-2">
                <small class="text-muted">{{ config.description || '暂无描述' }}</small>
                <div class="mt-2">
                  <span v-for="module in config.modules" :key="module" 
                        class="badge bg-info me-1 mb-1">{{ module }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Mihomo 配置编辑模态框 -->
    <div class="modal fade" id="mihomoConfigModal" tabindex="-1" ref="mihomoModalRef">
      <div class="modal-dialog modal-xl modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingMihomo.id ? '编辑' : '新增' }} Mihomo 配置</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <!-- 基本信息 -->
            <div class="row mb-4">
              <div class="col-md-6">
                <label class="form-label">配置名称 <span class="text-danger">*</span></label>
                <input type="text" class="form-control" v-model="editingMihomo.name" placeholder="例如：默认配置">
              </div>
              <div class="col-md-6">
                <label class="form-label">描述</label>
                <input type="text" class="form-control" v-model="editingMihomo.description" placeholder="配置描述（可选）">
              </div>
            </div>

            <!-- 配置模块手风琴 -->
            <div class="accordion" id="mihomoAccordion">
              <!-- 基础设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoBasic">
                    <i class="bi bi-gear me-2"></i>基础设置
                  </button>
                </h2>
                <div id="mihomoBasic" class="accordion-collapse collapse show" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div class="row g-3 mb-4">
                      <div class="col-md-2">
                        <label class="form-label">mixed-port</label>
                        <input type="number" class="form-control" v-model.number="editingMihomo.config.mixedPort" placeholder="7890">
                      </div>
                      <div class="col-md-2">
                        <label class="form-label">redir-port</label>
                        <input type="number" class="form-control" v-model.number="editingMihomo.config.redirPort" placeholder="9797">
                      </div>
                      <div class="col-md-2">
                        <label class="form-label">tproxy-port</label>
                        <input type="number" class="form-control" v-model.number="editingMihomo.config.tproxyPort" placeholder="9898">
                      </div>
                      <div class="col-md-2">
                        <label class="form-label">mode</label>
                        <select class="form-select" v-model="editingMihomo.config.mode">
                          <option value="rule">rule</option>
                          <option value="global">global</option>
                          <option value="direct">direct</option>
                        </select>
                      </div>
                      <div class="col-md-2">
                        <label class="form-label">bind-address</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.bindAddress" placeholder="*">
                      </div>
                      <div class="col-md-2">
                        <label class="form-label">log-level</label>
                        <select class="form-select" v-model="editingMihomo.config.logLevel">
                          <option value="silent">silent</option>
                          <option value="error">error</option>
                          <option value="warning">warning</option>
                          <option value="info">info</option>
                          <option value="debug">debug</option>
                        </select>
                      </div>
                    </div>
                    
                    <div class="row g-3 mb-4">
                      <div class="col-md-3">
                        <label class="form-label">external-controller</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.externalController" placeholder="0.0.0.0:9090">
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">external-ui</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.externalUi" placeholder="./dashboard">
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">find-process-mode</label>
                        <select class="form-select" v-model="editingMihomo.config.findProcessMode">
                          <option value="strict">strict</option>
                          <option value="always">always</option>
                          <option value="off">off</option>
                        </select>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">global-client-fingerprint</label>
                        <select class="form-select" v-model="editingMihomo.config.globalClientFingerprint">
                          <option value="chrome">chrome</option>
                          <option value="firefox">firefox</option>
                          <option value="safari">safari</option>
                          <option value="iOS">iOS</option>
                          <option value="android">android</option>
                          <option value="edge">edge</option>
                          <option value="random">random</option>
                        </select>
                      </div>
                    </div>
                    
                    <div class="row g-3 mb-4">
                      <div class="col-12">
                        <label class="form-label">external-ui-url</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.externalUiUrl" placeholder="https://github.com/MetaCubeX/metacubexd/archive/refs/heads/gh-pages.zip">
                      </div>
                    </div>
                    
                    <h6 class="mb-3">功能开关</h6>
                    <div class="row g-3 mb-4">
                      <div class="col-12">
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.ipv6" id="mihomoIpv6">
                          <label class="form-check-label" for="mihomoIpv6">ipv6</label>
                        </div>
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.allowLan" id="mihomoAllowLan">
                          <label class="form-check-label" for="mihomoAllowLan">allow-lan</label>
                        </div>
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.unifiedDelay" id="mihomoUnifiedDelay">
                          <label class="form-check-label" for="mihomoUnifiedDelay">unified-delay</label>
                        </div>
                        <div class="form-check form-switch d-inline-block">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.tcpConcurrent" id="mihomoTcpConcurrent">
                          <label class="form-check-label" for="mihomoTcpConcurrent">tcp-concurrent</label>
                        </div>
                      </div>
                    </div>
                    
                    <h6 class="mb-3">profile</h6>
                    <div class="row g-3">
                      <div class="col-12">
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.profile.storeSelected" id="mihomoStoreSelected">
                          <label class="form-check-label" for="mihomoStoreSelected">store-selected</label>
                        </div>
                        <div class="form-check form-switch d-inline-block">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.profile.storeFakeip" id="mihomoStoreFakeip">
                          <label class="form-check-label" for="mihomoStoreFakeip">store-fake-ip</label>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Sniffer 嗅探设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoSniffer">
                    <i class="bi bi-search me-2"></i>Sniffer 嗅探
                  </button>
                </h2>
                <div id="mihomoSniffer" class="accordion-collapse collapse" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div class="row g-3 mb-3">
                      <div class="col-12">
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.sniffer.enable" id="snifferEnable">
                          <label class="form-check-label" for="snifferEnable">enable</label>
                        </div>
                      </div>
                    </div>
                    <h6 class="mb-3">sniff</h6>
                    <div class="row g-3 mb-3">
                      <div class="col-md-4">
                        <label class="form-label">HTTP ports</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.sniffer.sniff.HTTP.ports" placeholder="80, 8080-8880">
                      </div>
                      <div class="col-md-2 d-flex align-items-end">
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.sniffer.sniff.HTTP.overrideDestination" id="httpOverride">
                          <label class="form-check-label" for="httpOverride">override-destination</label>
                        </div>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">TLS ports</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.sniffer.sniff.TLS.ports" placeholder="443, 8443">
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">QUIC ports</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.sniffer.sniff.QUIC.ports" placeholder="443, 8443">
                      </div>
                    </div>
                    <div class="row g-3">
                      <div class="col-12">
                        <label class="form-label">skip-domain <small class="text-muted">(每行一个)</small></label>
                        <textarea class="form-control" rows="2" v-model="editingMihomo.config.sniffer.skipDomain" placeholder="Mijia Cloud&#10;+.push.apple.com"></textarea>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- TUN 设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoTun">
                    <i class="bi bi-hdd-network me-2"></i>TUN 隧道
                  </button>
                </h2>
                <div id="mihomoTun" class="accordion-collapse collapse" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div class="row g-3">
                      <div class="col-12">
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.tun.enable" id="tunEnable">
                          <label class="form-check-label" for="tunEnable">enable</label>
                        </div>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">device</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.tun.device" placeholder="meta">
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">stack</label>
                        <select class="form-select" v-model="editingMihomo.config.tun.stack">
                          <option value="gvisor">gvisor</option>
                          <option value="system">system</option>
                          <option value="mixed">mixed</option>
                        </select>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">udp-timeout <small class="text-muted">(秒)</small></label>
                        <input type="number" class="form-control" v-model.number="editingMihomo.config.tun.udpTimeout" placeholder="300">
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">dns-hijack</label>
                        <textarea class="form-control" rows="2" v-model="editingMihomo.config.tun.dnsHijack" placeholder="any:53&#10;tcp://any:53"></textarea>
                      </div>
                      <div class="col-12">
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.tun.autoRoute" id="tunAutoRoute">
                          <label class="form-check-label" for="tunAutoRoute">auto-route</label>
                        </div>
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.tun.autoRedirect" id="tunAutoRedirect">
                          <label class="form-check-label" for="tunAutoRedirect">auto-redirect</label>
                        </div>
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.tun.autoDetectInterface" id="tunAutoDetect">
                          <label class="form-check-label" for="tunAutoDetect">auto-detect-interface</label>
                        </div>
                        <div class="form-check form-switch d-inline-block">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.tun.strictRoute" id="tunStrictRoute">
                          <label class="form-check-label" for="tunStrictRoute">strict-route</label>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- DNS 设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoDns">
                    <i class="bi bi-globe me-2"></i>DNS 配置
                  </button>
                </h2>
                <div id="mihomoDns" class="accordion-collapse collapse" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div class="row g-3 mb-4">
                      <div class="col-12">
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.dns.enable" id="dnsEnable">
                          <label class="form-check-label" for="dnsEnable">enable</label>
                        </div>
                        <div class="form-check form-switch d-inline-block">
                          <input class="form-check-input" type="checkbox" v-model="editingMihomo.config.dns.ipv6" id="dnsIpv6">
                          <label class="form-check-label" for="dnsIpv6">ipv6</label>
                        </div>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">listen</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.dns.listen" placeholder="0.0.0.0:1053">
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">enhanced-mode</label>
                        <select class="form-select" v-model="editingMihomo.config.dns.enhancedMode">
                          <option value="fake-ip">fake-ip</option>
                          <option value="redir-host">redir-host</option>
                        </select>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">fake-ip-range</label>
                        <input type="text" class="form-control" v-model="editingMihomo.config.dns.fakeIpRange" placeholder="198.18.0.1/16">
                      </div>
                    </div>
                    
                    <div class="row g-3 mb-4">
                      <div class="col-md-6">
                        <label class="form-label">default-nameserver <small class="text-muted">(解析 DNS 域名)</small></label>
                        <textarea class="form-control" rows="2" v-model="editingMihomo.config.dns.defaultNameserver" placeholder="223.5.5.5"></textarea>
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">nameserver <small class="text-muted">(默认 DNS 服务器)</small></label>
                        <textarea class="form-control" rows="2" v-model="editingMihomo.config.dns.nameserver" placeholder="8.8.8.8"></textarea>
                      </div>
                    </div>
                    
                    <div class="row g-3 mb-4">
                      <div class="col-md-6">
                        <label class="form-label">proxy-server-nameserver <small class="text-muted">(代理节点域名解析)</small></label>
                        <textarea class="form-control" rows="2" v-model="editingMihomo.config.dns.proxyServerNameserver" placeholder="223.5.5.5"></textarea>
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">nameserver-policy <small class="text-muted">(域名策略，每行一个 "规则: DNS")</small></label>
                        <textarea class="form-control" rows="2" v-model="editingMihomo.config.dns.nameserverPolicy" placeholder="rule-set:cn_domain: 223.5.5.5"></textarea>
                      </div>
                    </div>
                    
                    <div class="row g-3">
                      <div class="col-12">

                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 策略组设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoProxyGroups">
                    <i class="bi bi-diagram-3 me-2"></i>策略组
                  </button>
                </h2>
                <div id="mihomoProxyGroups" class="accordion-collapse collapse" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div v-for="(group, idx) in editingMihomo.config.proxyGroups" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'mihomoProxyGroups')"
                         @dragover.prevent="onDragOver($event, idx, 'mihomoProxyGroups')"
                         @drop="onDrop($event, 'mihomoProxyGroups')"
                         @dragend="onDragEnd"
                         @click="openProxyGroupEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="item-name me-3">{{ group.name || '未命名' }}</span>
                        <span class="badge bg-primary me-2">{{ group.type }}</span>
                        <span class="badge bg-secondary me-2">{{ group.filterMode }}</span>
                        <span class="item-detail text-muted text-truncate">{{ group.filter || '' }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeProxyGroup(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addProxyGroup">
                      <i class="bi bi-plus me-1"></i>添加策略组
                    </button>
                  </div>
                </div>
              </div>

              <!-- 规则集设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoRuleProviders">
                    <i class="bi bi-collection me-2"></i>规则集
                  </button>
                </h2>
                <div id="mihomoRuleProviders" class="accordion-collapse collapse" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div v-for="(provider, idx) in editingMihomo.config.ruleProviders" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'mihomoRuleProviders')"
                         @dragover.prevent="onDragOver($event, idx, 'mihomoRuleProviders')"
                         @drop="onDrop($event, 'mihomoRuleProviders')"
                         @dragend="onDragEnd"
                         @click="openRuleProviderEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="item-name me-3">{{ provider.name || '未命名' }}</span>
                        <span class="badge bg-info me-2">{{ provider.type }}</span>
                        <span class="badge bg-secondary me-2">{{ provider.behavior }}</span>
                        <span class="item-detail text-muted text-truncate">{{ provider.url || '' }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeRuleProvider(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addRuleProvider">
                      <i class="bi bi-plus me-1"></i>添加规则集
                    </button>
                  </div>
                </div>
              </div>

              <!-- 规则路由设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#mihomoRules">
                    <i class="bi bi-signpost-split me-2"></i>规则路由
                  </button>
                </h2>
                <div id="mihomoRules" class="accordion-collapse collapse" data-bs-parent="#mihomoAccordion">
                  <div class="accordion-body">
                    <div v-for="(rule, idx) in editingMihomo.config.rules" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'mihomoRules')"
                         @dragover.prevent="onDragOver($event, idx, 'mihomoRules')"
                         @drop="onDrop($event, 'mihomoRules')"
                         @dragend="onDragEnd"
                         @click="openMihomoRuleEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="item-detail text-truncate">{{ formatMihomoRule(rule) }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeMihomoRule(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addMihomoRule">
                      <i class="bi bi-plus me-1"></i>添加路由规则
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveMihomoConfig">
              <i class="bi bi-check-lg me-1"></i>保存配置
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox 配置编辑模态框 -->
    <div class="modal fade" id="singboxConfigModal" tabindex="-1" ref="singboxModalRef">
      <div class="modal-dialog modal-xl modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingSingBox.id ? '编辑' : '新增' }} SingBox 配置</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <!-- 基本信息 -->
            <div class="row mb-4">
              <div class="col-md-6">
                <label class="form-label">配置名称 <span class="text-danger">*</span></label>
                <input type="text" class="form-control" v-model="editingSingBox.name" placeholder="例如：默认配置">
              </div>
              <div class="col-md-6">
                <label class="form-label">描述</label>
                <input type="text" class="form-control" v-model="editingSingBox.description" placeholder="配置描述（可选）">
              </div>
            </div>

            <!-- 配置模块手风琴 -->
            <div class="accordion" id="singboxAccordion">
              <!-- Log 日志设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button" type="button" data-bs-toggle="collapse" data-bs-target="#singboxLog">
                    <i class="bi bi-journal-text me-2"></i>Log 日志
                  </button>
                </h2>
                <div id="singboxLog" class="accordion-collapse collapse show" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div class="row g-3">
                      <div class="col-md-6">
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.log.disabled" id="sbLogDisabled">
                          <label class="form-check-label" for="sbLogDisabled">禁用日志</label>
                        </div>
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">日志级别</label>
                        <select class="form-select" v-model="editingSingBox.config.log.level">
                          <option value="trace">trace</option>
                          <option value="debug">debug</option>
                          <option value="info">info</option>
                          <option value="warn">warn</option>
                          <option value="error">error</option>
                          <option value="fatal">fatal</option>
                          <option value="panic">panic</option>
                        </select>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- DNS 设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxDns">
                    <i class="bi bi-globe me-2"></i>DNS 配置
                  </button>
                </h2>
                <div id="singboxDns" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div class="row g-3">
                      <div class="col-md-3">
                        <label class="form-label">DNS 策略</label>
                        <select class="form-select" v-model="editingSingBox.config.dns.strategy">
                          <option value="prefer_ipv4">prefer_ipv4</option>
                          <option value="prefer_ipv6">prefer_ipv6</option>
                          <option value="ipv4_only">ipv4_only</option>
                          <option value="ipv6_only">ipv6_only</option>
                        </select>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">最终 DNS (final)</label>
                        <select class="form-select" v-model="editingSingBox.config.dns.final">
                          <option value="">不设置</option>
                          <option v-for="srv in editingSingBox.config.dns.servers" :key="srv.tag" :value="srv.tag">{{ srv.tag }}</option>
                        </select>
                      </div>
                      <div class="col-md-3">
                        <label class="form-label">客户端子网</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.dns.clientSubnet" placeholder="如: 223.5.5.0/24">
                      </div>
                      <div class="col-md-3 d-flex flex-column justify-content-end">
                        <div class="form-check form-switch">

                          <label class="form-check-label" for="sbDnsIndCache">独立缓存</label>
                        </div>
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.dns.reverseMapping" id="sbDnsReverseMap">
                          <label class="form-check-label" for="sbDnsReverseMap">反向映射</label>
                        </div>
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.dns.disableCache" id="sbDnsDisableCache">
                          <label class="form-check-label" for="sbDnsDisableCache">禁用缓存</label>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- DNS 服务器 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxDnsServers">
                    <i class="bi bi-server me-2"></i>DNS 服务器
                  </button>
                </h2>
                <div id="singboxDnsServers" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div v-for="(server, idx) in editingSingBox.config.dns.servers" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'singboxDnsServers')"
                         @dragover.prevent="onDragOver($event, idx, 'singboxDnsServers')"
                         @drop="onDrop($event, 'singboxDnsServers')"
                         @dragend="onDragEnd"
                         @click="openDnsServerEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="item-name me-3">{{ server.tag || '未命名' }}</span>
                        <span class="badge bg-primary me-2">{{ server.type }}</span>
                        <span class="item-detail text-muted text-truncate">{{ server.type === 'hosts' ? (server.predefined ? Object.keys(server.predefined).length + ' 个域名' : '') : (server.server || '') }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeSingBoxDnsServer(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addSingBoxDnsServer">
                      <i class="bi bi-plus me-1"></i>添加 DNS 服务器
                    </button>
                  </div>
                </div>
              </div>

              <!-- DNS 规则 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxDnsRules">
                    <i class="bi bi-list-check me-2"></i>DNS 规则
                  </button>
                </h2>
                <div id="singboxDnsRules" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div v-for="(rule, idx) in editingSingBox.config.dns.rules" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'singboxDnsRules')"
                         @dragover.prevent="onDragOver($event, idx, 'singboxDnsRules')"
                         @drop="onDrop($event, 'singboxDnsRules')"
                         @dragend="onDragEnd"
                         @click="openDnsRuleEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="badge bg-secondary me-2">{{ rule.type }}</span>
                        <span class="item-detail text-muted text-truncate me-2">{{ rule.type === 'rule_set' ? (rule.values?.join(', ') || '') : (rule.value || '') }}</span>
                        <span class="badge bg-info">{{ rule.server }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeSingBoxDnsRule(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addSingBoxDnsRule">
                      <i class="bi bi-plus me-1"></i>添加 DNS 规则
                    </button>
                  </div>
                </div>
              </div>

              <!-- Inbound 设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxInbound">
                    <i class="bi bi-box-arrow-in-right me-2"></i>Inbound 入站
                  </button>
                </h2>
                <div id="singboxInbound" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div class="row g-3">
                      <div class="col-12">
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.inbound.tunEnable" id="sbTunEnable">
                          <label class="form-check-label" for="sbTunEnable">启用 TUN</label>
                        </div>
                      </div>
                      <div class="col-md-4">
                        <label class="form-label">接口名称</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.inbound.interfaceName" placeholder="momo">
                      </div>
                      <div class="col-md-4">
                        <label class="form-label">协议栈</label>
                        <select class="form-select" v-model="editingSingBox.config.inbound.stack">
                          <option value="gvisor">gVisor</option>
                          <option value="system">System</option>
                          <option value="mixed">Mixed</option>
                        </select>
                      </div>
                      <div class="col-md-4">
                        <label class="form-label">MTU</label>
                        <input type="number" class="form-control" v-model.number="editingSingBox.config.inbound.mtu" placeholder="9000">
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">IPv4 地址</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.inbound.addressIpv4" placeholder="172.19.0.1/30">
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">IPv6 地址</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.inbound.addressIpv6" placeholder="fdfe:dcba:9876::1/126">
                      </div>
                      <div class="col-12">
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.inbound.autoRoute" id="sbAutoRoute">
                          <label class="form-check-label" for="sbAutoRoute">自动路由</label>
                        </div>
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.inbound.autoRedirect" id="sbAutoRedirect">
                          <label class="form-check-label" for="sbAutoRedirect">自动重定向</label>
                        </div>
                        <div class="form-check form-switch d-inline-block">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.inbound.strictRoute" id="sbStrictRoute">
                          <label class="form-check-label" for="sbStrictRoute">严格路由</label>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Outbound 策略组设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxOutbound">
                    <i class="bi bi-box-arrow-right me-2"></i>Outbound 策略组
                  </button>
                </h2>
                <div id="singboxOutbound" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div v-for="(group, idx) in editingSingBox.config.outboundGroups" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'singboxOutboundGroups')"
                         @dragover.prevent="onDragOver($event, idx, 'singboxOutboundGroups')"
                         @drop="onDrop($event, 'singboxOutboundGroups')"
                         @dragend="onDragEnd"
                         @click="openSingBoxGroupEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="item-name me-3">{{ group.tag || '未命名' }}</span>
                        <span class="badge bg-primary me-2">{{ group.type }}</span>
                        <span class="badge bg-secondary me-2">{{ group.filterMode }}</span>
                        <span class="item-detail text-muted text-truncate">{{ group.filter || '' }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeSingBoxGroup(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addSingBoxGroup">
                      <i class="bi bi-plus me-1"></i>添加策略组
                    </button>
                  </div>
                </div>
              </div>

              <!-- Route 路由设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxRoute">
                    <i class="bi bi-signpost-split me-2"></i>Route 路由
                  </button>
                </h2>
                <div id="singboxRoute" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div class="row g-3">
                      <div class="col-md-4">
                        <label class="form-label">默认出站 (final)</label>
                        <select class="form-select" v-model="editingSingBox.config.route.final">
                          <option value="">select final...</option>
                          <option value="直连">直连</option>
                          <option v-for="group in editingSingBox.config.outboundGroups" :key="group.tag" :value="group.tag">{{ group.tag }}</option>
                        </select>
                      </div>
                      <div class="col-md-4">
                        <label class="form-label">默认域名解析器</label>
                        <select class="form-select" v-model="editingSingBox.config.route.defaultDomainResolver">
                          <option value="">select server...</option>
                          <option v-for="srv in editingSingBox.config.dns.servers" :key="srv.tag" :value="srv.tag">{{ srv.tag }}</option>
                        </select>
                      </div>
                      <div class="col-md-4 d-flex align-items-end">
                        <div class="form-check form-switch">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.route.autoDetectInterface" id="sbRouteAutoDetect">
                          <label class="form-check-label" for="sbRouteAutoDetect">自动检测接口</label>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 规则集 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxRuleSets">
                    <i class="bi bi-collection me-2"></i>规则集
                  </button>
                </h2>
                <div id="singboxRuleSets" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div v-for="(ruleSet, idx) in editingSingBox.config.route.ruleSets" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'singboxRuleSets')"
                         @dragover.prevent="onDragOver($event, idx, 'singboxRuleSets')"
                         @drop="onDrop($event, 'singboxRuleSets')"
                         @dragend="onDragEnd"
                         @click="openSingBoxRuleSetEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="item-name me-3">{{ ruleSet.tag || '未命名' }}</span>
                        <span class="badge bg-info me-2">{{ ruleSet.type }}</span>
                        <span class="item-detail text-muted text-truncate">{{ ruleSet.url || '' }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeSingBoxRuleSet(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addSingBoxRuleSet">
                      <i class="bi bi-plus me-1"></i>添加规则集
                    </button>
                  </div>
                </div>
              </div>

              <!-- 路由规则 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxRouteRules">
                    <i class="bi bi-diagram-3 me-2"></i>路由规则
                  </button>
                </h2>
                <div id="singboxRouteRules" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div v-for="(rule, idx) in editingSingBox.config.route.rules" :key="idx" 
                         class="item-card mb-2 draggable-item"
                         draggable="true"
                         @dragstart="onDragStart($event, idx, 'singboxRouteRules')"
                         @dragover.prevent="onDragOver($event, idx, 'singboxRouteRules')"
                         @drop="onDrop($event, 'singboxRouteRules')"
                         @dragend="onDragEnd"
                         @click="openSingBoxRouteRuleEditor(idx)">
                      <div class="d-flex align-items-center">
                        <i class="bi bi-grip-vertical drag-handle text-muted me-2" style="cursor: grab;"></i>
                        <span class="badge bg-secondary me-2">{{ rule.type }}</span>
                        <span class="item-detail text-muted text-truncate me-2">{{ formatSingBoxRouteRule(rule) }}</span>
                        <span v-if="rule.action" class="badge bg-warning me-1">{{ rule.action }}</span>
                        <span v-if="rule.outbound" class="badge bg-info">{{ rule.outbound }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeSingBoxRouteRule(idx)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                    <button class="btn btn-outline-primary btn-sm" @click="addSingBoxRouteRule">
                      <i class="bi bi-plus me-1"></i>添加路由规则
                    </button>
                  </div>
                </div>
              </div>

              <!-- Experimental 实验性设置 -->
              <div class="accordion-item">
                <h2 class="accordion-header">
                  <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#singboxExperimental">
                    <i class="bi bi-lightning me-2"></i>Experimental 实验性
                  </button>
                </h2>
                <div id="singboxExperimental" class="accordion-collapse collapse" data-bs-parent="#singboxAccordion">
                  <div class="accordion-body">
                    <div class="row g-3">
                      <div class="col-12">
                        <h6>缓存文件</h6>
                        <div class="form-check form-switch d-inline-block me-4">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.experimental.cacheFileEnabled" id="sbCacheEnable">
                          <label class="form-check-label" for="sbCacheEnable">启用缓存</label>
                        </div>
                        <div class="form-check form-switch d-inline-block">
                          <input class="form-check-input" type="checkbox" v-model="editingSingBox.config.experimental.storeFakeip" id="sbStoreFakeip">
                          <label class="form-check-label" for="sbStoreFakeip">存储 Fake-IP</label>
                        </div>
                      </div>
                      <div class="col-12 mt-3">
                        <h6>Clash API</h6>
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">外部控制器</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.experimental.externalController" placeholder="0.0.0.0:9090">
                      </div>
                      <div class="col-md-6">
                        <label class="form-label">外部 UI</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.experimental.externalUi" placeholder="ui">
                      </div>
                      <div class="col-12">
                        <label class="form-label">外部 UI 下载地址</label>
                        <input type="text" class="form-control" v-model="editingSingBox.config.experimental.externalUiDownloadUrl" placeholder="https://...">
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveSingBoxConfig">
              <i class="bi bi-check-lg me-1"></i>保存配置
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Mihomo 策略组编辑弹框 -->
    <div class="modal fade" id="proxyGroupEditorModal" tabindex="-1" ref="proxyGroupEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑策略组</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingProxyGroup">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">名称</label>
                <input type="text" class="form-control" v-model="editingProxyGroup.name" placeholder="策略组名称">
              </div>
              <div class="col-md-6">
                <label class="form-label">类型</label>
                <select class="form-select" v-model="editingProxyGroup.type">
                  <option value="select">select (手动选择)</option>
                  <option value="url-test">url-test (自动测速)</option>
                  <option value="fallback">fallback (自动回退)</option>
                  <option value="load-balance">load-balance (负载均衡)</option>
                  <option value="relay">relay (链式代理)</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">过滤模式</label>
                <select class="form-select" v-model="editingProxyGroup.filterMode">
                  <option value="regex">regex (正则匹配)</option>
                  <option value="geoip-cn">geoip-cn (CN/非CN)</option>
                  <option value="geoip-country">geoip-country (国家)</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">过滤规则</label>
                <select v-if="editingProxyGroup.filterMode === 'geoip-cn'" class="form-select" v-model="editingProxyGroup.filter">
                  <option value="cn">cn (国内)</option>
                  <option value="!cn">!cn (非国内)</option>
                </select>
                <input v-else type="text" class="form-control" v-model="editingProxyGroup.filter" 
                       :placeholder="editingProxyGroup.filterMode === 'geoip-country' ? 'US,GB,JP...' : 'regex filter'">
              </div>
              <div class="col-md-6">
                <label class="form-label">排除过滤</label>
                <input type="text" class="form-control" v-model="editingProxyGroup.excludeFilter" placeholder="exclude-filter">
              </div>
              <div class="col-md-6">
                <label class="form-label">选项</label>
                <div class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="editingProxyGroup.includeAll" id="pgIncludeAll">
                  <label class="form-check-label" for="pgIncludeAll">include-all (包含所有节点)</label>
                </div>
              </div>
              <template v-if="editingProxyGroup.type !== 'select' && editingProxyGroup.type !== 'relay'">
                <div class="col-md-6">
                  <label class="form-label">测速 URL</label>
                  <input type="text" class="form-control" v-model="editingProxyGroup.url" placeholder="http://www.gstatic.com/generate_204">
                </div>
                <div class="col-md-3">
                  <label class="form-label">间隔(秒)</label>
                  <input type="number" class="form-control" v-model.number="editingProxyGroup.interval" placeholder="300">
                </div>
                <div class="col-md-3">
                  <label class="form-label">超时(ms)</label>
                  <input type="number" class="form-control" v-model.number="editingProxyGroup.timeout" placeholder="5000">
                </div>
                <div class="col-md-4" v-if="editingProxyGroup.type === 'url-test'">
                  <label class="form-label">容差(ms)</label>
                  <input type="number" class="form-control" v-model.number="editingProxyGroup.tolerance" placeholder="50">
                </div>
                <div class="col-md-4" v-if="editingProxyGroup.type === 'load-balance'">
                  <label class="form-label">负载策略</label>
                  <select class="form-select" v-model="editingProxyGroup.strategy">
                    <option value="">默认</option>
                    <option value="round-robin">round-robin</option>
                    <option value="consistent-hashing">consistent-hashing</option>
                    <option value="sticky-sessions">sticky-sessions</option>
                  </select>
                </div>
                <div class="col-md-4">
                  <div class="form-check form-switch mt-4">
                    <input class="form-check-input" type="checkbox" v-model="editingProxyGroup.lazy" id="pgLazy">
                    <label class="form-check-label" for="pgLazy">lazy</label>
                  </div>
                </div>
                <div class="col-md-4">
                  <div class="form-check form-switch mt-4">
                    <input class="form-check-input" type="checkbox" v-model="editingProxyGroup.hidden" id="pgHidden">
                    <label class="form-check-label" for="pgHidden">hidden</label>
                  </div>
                </div>
              </template>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Mihomo 规则集编辑弹框 -->
    <div class="modal fade" id="ruleProviderEditorModal" tabindex="-1" ref="ruleProviderEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑规则集</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingRuleProvider">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">名称</label>
                <input type="text" class="form-control" v-model="editingRuleProvider.name" placeholder="规则集名称">
              </div>
              <div class="col-md-6">
                <label class="form-label">类型</label>
                <select class="form-select" v-model="editingRuleProvider.type">
                  <option value="http">http (远程)</option>
                  <option value="file">file (本地)</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Behavior</label>
                <select class="form-select" v-model="editingRuleProvider.behavior">
                  <option value="domain">domain</option>
                  <option value="ipcidr">ipcidr</option>
                  <option value="classical">classical</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">Format</label>
                <select class="form-select" v-model="editingRuleProvider.format">
                  <option value="yaml">yaml</option>
                  <option value="text">text</option>
                  <option value="mrs">mrs</option>
                </select>
              </div>
              <div class="col-12">
                <label class="form-label">URL</label>
                <input type="text" class="form-control" v-model="editingRuleProvider.url" placeholder="规则集URL">
              </div>
              <div class="col-md-6">
                <label class="form-label">更新间隔(秒)</label>
                <input type="number" class="form-control" v-model.number="editingRuleProvider.interval" placeholder="86400">
              </div>
              <div class="col-md-6">
                <label class="form-label">代理</label>
                <select class="form-select" v-model="editingRuleProvider.proxy">
                  <option value="">默认</option>
                  <option value="DIRECT">DIRECT</option>
                  <option v-for="group in editingMihomo.config.proxyGroups" :key="group.name" :value="group.name">{{ group.name }}</option>
                </select>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Mihomo 路由规则编辑弹框 -->
    <div class="modal fade" id="mihomoRuleEditorModal" tabindex="-1" ref="mihomoRuleEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑路由规则</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingMihomoRule">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">规则类型</label>
                <select class="form-select" v-model="editingMihomoRule.type" @change="onEditingMihomoRuleTypeChange">
                  <option value="RULE-SET">RULE-SET</option>
                  <option value="DOMAIN">DOMAIN</option>
                  <option value="DOMAIN-SUFFIX">DOMAIN-SUFFIX</option>
                  <option value="DOMAIN-KEYWORD">DOMAIN-KEYWORD</option>
                  <option value="DOMAIN-REGEX">DOMAIN-REGEX</option>
                  <option value="GEOSITE">GEOSITE</option>
                  <option value="GEOIP">GEOIP</option>
                  <option value="IP-CIDR">IP-CIDR</option>
                  <option value="IP-ASN">IP-ASN</option>
                  <option value="SRC-IP-CIDR">SRC-IP-CIDR</option>
                  <option value="DST-PORT">DST-PORT</option>
                  <option value="SRC-PORT">SRC-PORT</option>
                  <option value="PROCESS-NAME">PROCESS-NAME</option>
                  <option value="PROCESS-PATH">PROCESS-PATH</option>
                  <option value="NETWORK">NETWORK</option>
                  <option value="AND">AND</option>
                  <option value="OR">OR</option>
                  <option value="NOT">NOT</option>
                  <option value="MATCH">MATCH</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">出站</label>
                <select class="form-select" v-model="editingMihomoRule.outbound">
                  <option value="">选择出站...</option>
                  <option value="DIRECT">DIRECT</option>
                  <option value="REJECT">REJECT</option>
                  
                  <option v-for="group in editingMihomo.config.proxyGroups" :key="group.name" :value="group.name">{{ group.name }}</option>
                </select>
              </div>
              <div class="col-12" v-if="!['AND', 'OR', 'NOT', 'MATCH'].includes(editingMihomoRule.type)">
                <label class="form-label">值</label>
                <select v-if="editingMihomoRule.type === 'RULE-SET'" class="form-select" v-model="editingMihomoRule.value">
                  <option value="">选择规则集...</option>
                  <option v-for="rp in editingMihomo.config.ruleProviders" :key="rp.name" :value="rp.name">{{ rp.name }}</option>
                </select>
                <select v-else-if="editingMihomoRule.type === 'NETWORK'" class="form-select" v-model="editingMihomoRule.value">
                  <option value="tcp">tcp</option>
                  <option value="udp">udp</option>
                </select>
                <input v-else type="text" class="form-control" v-model="editingMihomoRule.value" 
                       :placeholder="getMihomoRulePlaceholder(editingMihomoRule.type)">
              </div>
              <div class="col-12">
                <div class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="editingMihomoRule.noResolve" id="mrNoResolve">
                  <label class="form-check-label" for="mrNoResolve">no-resolve (跳过DNS解析)</label>
                </div>
              </div>
              <!-- 联合规则子规则 -->
              <div v-if="['AND', 'OR', 'NOT'].includes(editingMihomoRule.type)" class="col-12">
                <label class="form-label">子规则</label>
                <div v-for="(subRule, subIdx) in editingMihomoRule.subRules" :key="subIdx" class="row g-2 mb-2">
                  <div class="col-md-4">
                    <select class="form-select form-select-sm" v-model="subRule.type">
                      <option value="RULE-SET">RULE-SET</option>
                      <option value="DOMAIN">DOMAIN</option>
                      <option value="DOMAIN-SUFFIX">DOMAIN-SUFFIX</option>
                      <option value="GEOSITE">GEOSITE</option>
                      <option value="GEOIP">GEOIP</option>
                      <option value="IP-CIDR">IP-CIDR</option>
                      <option value="DST-PORT">DST-PORT</option>
                      <option value="NETWORK">NETWORK</option>
                    </select>
                  </div>
                  <div class="col-md-6">
                    <select v-if="subRule.type === 'RULE-SET'" class="form-select form-select-sm" v-model="subRule.value">
                      <option value="">选择规则集...</option>
                      <option v-for="rp in editingMihomo.config.ruleProviders" :key="rp.name" :value="rp.name">{{ rp.name }}</option>
                    </select>
                    <input v-else type="text" class="form-control form-control-sm" v-model="subRule.value" :placeholder="getMihomoRulePlaceholder(subRule.type)">
                  </div>
                  <div class="col-md-2">
                    <button class="btn btn-outline-danger btn-sm" @click="editingMihomoRule.subRules.splice(subIdx, 1)">
                      <i class="bi bi-x"></i>
                    </button>
                  </div>
                </div>
                <button class="btn btn-outline-secondary btn-sm" @click="editingMihomoRule.subRules.push({ type: 'DOMAIN-SUFFIX', value: '' })">
                  <i class="bi bi-plus me-1"></i>添加子规则
                </button>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox DNS服务器编辑弹框 -->
    <div class="modal fade" id="dnsServerEditorModal" tabindex="-1" ref="dnsServerEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑 DNS 服务器</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingDnsServer">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">标签 (tag)</label>
                <input type="text" class="form-control" v-model="editingDnsServer.tag" placeholder="DNS服务器标签">
              </div>
              <div class="col-md-6">
                <label class="form-label">类型</label>
                <select class="form-select" v-model="editingDnsServer.type">
                  <option value="local">local (本地)</option>
                  <option value="hosts">hosts (预定义)</option>
                  <option value="udp">udp</option>
                  <option value="tcp">tcp</option>
                  <option value="https">https (DoH)</option>
                  <option value="tls">tls (DoT)</option>
                  <option value="quic">quic (DoQ)</option>
                  <option value="h3">h3 (HTTP/3)</option>
                  <option value="fakeip">fakeip</option>
                </select>
              </div>
              <template v-if="editingDnsServer.type === 'fakeip'">
                <div class="col-md-6">
                  <label class="form-label">IPv4 范围</label>
                  <input type="text" class="form-control" v-model="editingDnsServer.inet4Range" placeholder="198.18.0.0/15">
                </div>
                <div class="col-md-6">
                  <label class="form-label">IPv6 范围</label>
                  <input type="text" class="form-control" v-model="editingDnsServer.inet6Range" placeholder="fc00::/18">
                </div>
              </template>
              <template v-else-if="editingDnsServer.type === 'hosts'">
                <div class="col-12">
                  <label class="form-label mb-2">域名映射</label>
                  <div v-if="editingDnsServer.predefined && Object.keys(editingDnsServer.predefined).length > 0">
                    <div v-for="(ips, domain) in editingDnsServer.predefined" :key="domain"
                         class="item-card mb-2 draggable-item" style="cursor:pointer" @click="openHostEntryEditor(domain)">
                      <div class="d-flex align-items-center">
                        <span class="fw-medium me-2" style="min-width:140px">{{ domain }}</span>
                        <span class="text-muted small text-truncate">{{ ips.join(', ') }}</span>
                        <button class="btn btn-sm btn-outline-danger ms-auto" @click.stop="removeHostEntry(domain)">
                          <i class="bi bi-x"></i>
                        </button>
                      </div>
                    </div>
                  </div>
                  <div v-else class="text-muted small mb-2 py-2">
                    暂无域名映射
                  </div>
                  <button class="btn btn-outline-primary btn-sm" @click="openHostEntryEditor('')">
                    <i class="bi bi-plus me-1"></i>添加域名
                  </button>
                </div>
              </template>
              <template v-else>
                <div class="col-md-8">
                  <label class="form-label">服务器地址</label>
                  <input type="text" class="form-control" v-model="editingDnsServer.server" 
                         :placeholder="editingDnsServer.type === 'local' ? 'N/A' : '服务器地址'"
                         :disabled="editingDnsServer.type === 'local'">
                </div>
                <div class="col-md-4">
                  <label class="form-label">端口</label>
                  <input type="number" class="form-control" v-model.number="editingDnsServer.serverPort" 
                         :placeholder="editingDnsServer.type === 'https' || editingDnsServer.type === 'h3' ? '443' : '53'"
                         :disabled="editingDnsServer.type === 'local'">
                </div>
              </template>
              <div class="col-md-6">
                <label class="form-label">出站代理 (detour)</label>
                <input type="text" class="form-control" v-model="editingDnsServer.detour" placeholder="可选，用于指定DNS请求的出站">
              </div>
              <div class="col-md-6">
                <label class="form-label">域名解析器 (domain_resolver)</label>
                <select class="form-select" v-model="editingDnsServer.domainResolver">
                  <option value="">不设置</option>
                  <option v-for="srv in editingSingBox.config.dns.servers.filter(s => s.tag !== editingDnsServer?.tag)" :key="srv.tag" :value="srv.tag">{{ srv.tag }}</option>
                </select>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 域名条目编辑弹框 -->
    <div class="modal fade" id="hostEntryEditorModal" tabindex="-1" ref="hostEntryEditorRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ editingHostEntryDomain !== null ? '编辑域名映射' : '添加域名映射' }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="row g-3">
              <div class="col-12">
                <label class="form-label">域名</label>
                <input type="text" class="form-control" v-model="editingHostEntryDomainInput" placeholder="dns.alidns.com">
              </div>
              <div class="col-12">
                <label class="form-label">IP 地址</label>
                <input type="text" class="form-control" v-model="editingHostEntryIps" placeholder="223.5.5.5, 2400:3200::1">
                <div class="form-text">多个 IP 地址用英文逗号分隔</div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveHostEntry">保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox DNS规则编辑弹框 -->
    <div class="modal fade" id="dnsRuleEditorModal" tabindex="-1" ref="dnsRuleEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑 DNS 规则</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingDnsRule">
            <div class="row g-3">
              <div class="col-md-4">
                <label class="form-label">规则类型</label>
                <select class="form-select" v-model="editingDnsRule.type">
                  <option value="rule_set">rule_set</option>
                  <option value="clash_mode">clash_mode</option>
                  <option value="query_type">query_type</option>
                  <option value="domain">domain</option>
                  <option value="domain_suffix">domain_suffix</option>
                </select>
              </div>
              <div class="col-md-4">
                <label class="form-label">DNS 服务器</label>
                <select class="form-select" v-model="editingDnsRule.server">
                  <option value="">不设置</option>
                  <option v-for="srv in editingSingBox.config.dns.servers" :key="srv.tag" :value="srv.tag">{{ srv.tag }}</option>
                </select>
              </div>
              <div class="col-md-4">
                <label class="form-label">动作 (action)</label>
                <select class="form-select" v-model="editingDnsRule.action">
                  <option value="">不设置</option>
                  <option value="predefined">predefined</option>
                  <option value="reject">reject</option>
                </select>
              </div>
              <div class="col-12" v-if="editingDnsRule.type === 'rule_set'">
                <label class="form-label">规则集</label>
                <div class="d-flex flex-wrap gap-2 p-2 border rounded">
                  <span v-for="rs in editingSingBox.config.route.ruleSets" :key="rs.tag" 
                        class="badge cursor-pointer"
                        :class="editingDnsRule.values?.includes(rs.tag) ? 'bg-primary' : 'bg-secondary'"
                        @click="toggleEditingDnsRuleSet(rs.tag)"
                        style="cursor: pointer;">
                    {{ rs.tag }}
                  </span>
                </div>
              </div>
              <div class="col-12" v-else>
                <label class="form-label">值</label>
                <input type="text" class="form-control" v-model="editingDnsRule.value" placeholder="value">
              </div>
              <div class="col-12">
                <div class="form-check form-switch">
                  <input class="form-check-input" type="checkbox" v-model="editingDnsRule.rewriteTtl" id="drTtl">
                  <label class="form-check-label" for="drTtl">TTL=1</label>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox 策略组编辑弹框 -->
    <div class="modal fade" id="singboxGroupEditorModal" tabindex="-1" ref="singboxGroupEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑策略组</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingSingBoxGroup">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">标签 (tag)</label>
                <input type="text" class="form-control" v-model="editingSingBoxGroup.tag" placeholder="策略组标签">
              </div>
              <div class="col-md-6">
                <label class="form-label">类型</label>
                <select class="form-select" v-model="editingSingBoxGroup.type">
                  <option value="selector">selector (手动选择)</option>
                  <option value="urltest">urltest (自动测速)</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">过滤模式</label>
                <select class="form-select" v-model="editingSingBoxGroup.filterMode">
                  <option value="regex">regex</option>
                  <option value="geoip-cn">geoip-cn</option>
                  <option value="geoip-country">geoip-country</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">过滤规则</label>
                <select v-if="editingSingBoxGroup.filterMode === 'geoip-cn'" class="form-select" v-model="editingSingBoxGroup.filter">
                  <option value="cn">cn (国内)</option>
                  <option value="!cn">!cn (非国内)</option>
                </select>
                <input v-else type="text" class="form-control" v-model="editingSingBoxGroup.filter" 
                       :placeholder="editingSingBoxGroup.filterMode === 'geoip-country' ? 'US,GB,JP...' : 'regex pattern'">
              </div>
              <template v-if="editingSingBoxGroup.type === 'urltest'">
                <div class="col-md-8">
                  <label class="form-label">测速 URL</label>
                  <input type="text" class="form-control" v-model="editingSingBoxGroup.url" placeholder="https://www.gstatic.com/generate_204">
                </div>
                <div class="col-md-4">
                  <label class="form-label">间隔</label>
                  <input type="text" class="form-control" v-model="editingSingBoxGroup.interval" placeholder="3m">
                </div>
              </template>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox 路由规则编辑弹框 -->
    <div class="modal fade" id="singboxRouteRuleEditorModal" tabindex="-1" ref="singboxRouteRuleEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑路由规则</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingSingBoxRouteRule">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">规则类型</label>
                <select class="form-select" v-model="editingSingBoxRouteRule.type" @change="onEditingSingBoxRouteRuleTypeChange">
                  <option value="rule_set">rule_set</option>
                  <option value="clash_mode">clash_mode</option>
                  <option value="logical">logical</option>
                  <option value="domain">domain</option>
                  <option value="domain_suffix">domain_suffix</option>
                  <option value="domain_keyword">domain_keyword</option>
                  <option value="ip_cidr">ip_cidr</option>
                  <option value="port">port</option>
                  <option value="protocol">protocol</option>
                  <option value="ip_is_private">ip_is_private</option>
                </select>
              </div>
              <div class="col-md-3">
                <label class="form-label">动作</label>
                <select class="form-select" v-model="editingSingBoxRouteRule.action" :disabled="!!editingSingBoxRouteRule.outbound">
                  <option value="">无</option>
                  <option value="sniff">sniff</option>
                  <option value="reject">reject</option>
                  <option value="hijack-dns">hijack-dns</option>
                  <option value="resolve">resolve</option>
                </select>
              </div>
              <div class="col-md-3">
                <label class="form-label">出站</label>
                <select class="form-select" v-model="editingSingBoxRouteRule.outbound" :disabled="!!editingSingBoxRouteRule.action">
                  <option value="">无</option>
                  <option value="直连">直连</option>
                  <option v-for="group in editingSingBox.config.outboundGroups" :key="group.tag" :value="group.tag">{{ group.tag }}</option>
                </select>
              </div>
              <div class="col-12" v-if="editingSingBoxRouteRule.type === 'rule_set'">
                <label class="form-label">规则集</label>
                <div class="d-flex flex-wrap gap-2 p-2 border rounded">
                  <span v-for="rs in editingSingBox.config.route.ruleSets" :key="rs.tag" 
                        class="badge cursor-pointer"
                        :class="editingSingBoxRouteRule.values?.includes(rs.tag) ? 'bg-info' : 'bg-secondary'"
                        @click="toggleEditingRouteRuleSet(rs.tag)"
                        style="cursor: pointer;">
                    {{ rs.tag }}
                  </span>
                </div>
              </div>
              <div class="col-12" v-else-if="editingSingBoxRouteRule.type === 'logical'">
                <label class="form-label">逻辑模式</label>
                <select class="form-select mb-3" v-model="editingSingBoxRouteRule.mode">
                  <option value="or">or</option>
                  <option value="and">and</option>
                </select>
                <label class="form-label">子规则</label>
                <div v-for="(subRule, subIdx) in editingSingBoxRouteRule.subRules" :key="subIdx" class="row g-2 mb-2">
                  <div class="col-md-4">
                    <select class="form-select form-select-sm" v-model="subRule.type">
                      <option value="domain">domain</option>
                      <option value="domain_suffix">domain_suffix</option>
                      <option value="ip_cidr">ip_cidr</option>
                      <option value="port">port</option>
                      <option value="protocol">protocol</option>
                      <option value="rule_set">rule_set</option>
                    </select>
                  </div>
                  <div class="col-md-6">
                    <select v-if="subRule.type === 'rule_set'" class="form-select form-select-sm" v-model="subRule.value">
                      <option value="">选择规则集...</option>
                      <option v-for="rs in editingSingBox.config.route.ruleSets" :key="rs.tag" :value="rs.tag">{{ rs.tag }}</option>
                    </select>
                    <input v-else type="text" class="form-control form-control-sm" v-model="subRule.value" :placeholder="getSubRulePlaceholder(subRule.type)">
                  </div>
                  <div class="col-md-2">
                    <button class="btn btn-outline-danger btn-sm" @click="editingSingBoxRouteRule.subRules.splice(subIdx, 1)">
                      <i class="bi bi-x"></i>
                    </button>
                  </div>
                </div>
                <button class="btn btn-outline-secondary btn-sm" @click="editingSingBoxRouteRule.subRules.push({ type: 'domain_suffix', value: '' })">
                  <i class="bi bi-plus me-1"></i>添加子规则
                </button>
              </div>
              <div class="col-12" v-else-if="editingSingBoxRouteRule.type !== 'ip_is_private'">
                <label class="form-label">值</label>
                <input type="text" class="form-control" v-model="editingSingBoxRouteRule.value" placeholder="value (逗号分隔)">
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>

    <!-- SingBox 规则集编辑弹框 -->
    <div class="modal fade" id="singboxRuleSetEditorModal" tabindex="-1" ref="singboxRuleSetEditorRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">编辑规则集</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body" v-if="editingSingBoxRuleSet">
            <div class="row g-3">
              <div class="col-md-6">
                <label class="form-label">标签 (tag)</label>
                <input type="text" class="form-control" v-model="editingSingBoxRuleSet.tag" placeholder="规则集标签">
              </div>
              <div class="col-md-6">
                <label class="form-label">类型</label>
                <select class="form-select" v-model="editingSingBoxRuleSet.type">
                  <option value="remote">remote (远程)</option>
                  <option value="local">local (本地)</option>
                </select>
              </div>
              <div class="col-md-6">
                <label class="form-label">格式</label>
                <select class="form-select" v-model="editingSingBoxRuleSet.format">
                  <option value="binary">binary (.srs)</option>
                  <option value="source">source (JSON)</option>
                </select>
              </div>
              <div class="col-12">
                <label class="form-label">URL</label>
                <input type="text" class="form-control" v-model="editingSingBoxRuleSet.url" placeholder="规则集URL">
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Modal } from 'bootstrap'
import { getSubscriptionConfigs, saveSubscriptionConfigs } from '@/api'

// Mihomo 配置接口
interface MihomoConfig {
  id: number
  name: string
  description: string
  enabled: boolean
  modules: string[]
  config: {
    // 基础设置
    mixedPort: number
    redirPort: number
    tproxyPort: number
    mode: string
    bindAddress: string
    logLevel: string
    ipv6: boolean
    allowLan: boolean
    unifiedDelay: boolean
    tcpConcurrent: boolean
    externalController: string
    externalUi: string
    externalUiUrl: string
    findProcessMode: string
    globalClientFingerprint: string
    profile: {
      storeSelected: boolean
      storeFakeip: boolean
    }
    // Sniffer
    sniffer: {
      enable: boolean
      sniff: {
        HTTP: { ports: string; overrideDestination: boolean }
        TLS: { ports: string }
        QUIC: { ports: string }
      }
      skipDomain: string
    }
    // TUN
    tun: {
      enable: boolean
      device: string
      stack: string
      dnsHijack: string
      udpTimeout: number
      autoRoute: boolean
      autoRedirect: boolean
      autoDetectInterface: boolean
      strictRoute: boolean
    }
    // DNS
    dns: {
      enable: boolean
      ipv6: boolean
      listen: string
      enhancedMode: string
      fakeIpRange: string
      defaultNameserver: string
      nameserver: string
      proxyServerNameserver: string
      nameserverPolicy: string
    }
    // 策略组
    proxyGroups: Array<{
      name: string
      type: string
      filterMode: string
      filter: string
      excludeFilter: string
      includeAll: boolean
      url: string
      interval: number
      timeout: number
      tolerance: number
      lazy: boolean
      hidden: boolean
      strategy: string
    }>
    // 规则集
    ruleProviders: Array<{ name: string; type: string; behavior: string; format: string; url: string; interval: number; proxy: string }>
    // 路由规则
    rules: Array<{
      type: string
      value: string
      outbound: string
      noResolve: boolean
      subRules?: Array<{ type: string; value: string }>
    }>
  }
}

// SingBox 配置接口
interface SingBoxConfig {
  id: number
  name: string
  description: string
  enabled: boolean
  modules: string[]
  config: {
    log: {
      disabled: boolean
      level: string
      output: string
      timestamp: boolean
    }
    dns: {
      servers: Array<{ tag: string; type: string; server: string; serverPort: number; detour: string; domainResolver: string; inet4Range: string; inet6Range: string; predefined?: Record<string, string[]>; servers?: string[] }>
      strategy: string
      final: string
      clientSubnet: string
      optimistic: boolean
      reverseMapping: boolean
      disableCache: boolean
      cacheCapacity: number
      rules: Array<{ type: string; value: string; values?: string[]; server: string; action: string; rewriteTtl: boolean }>
    }
    ntp: {
      enabled: boolean
      interval: string
      server: string
      serverPort: number
    }
    inbound: {
      tunEnable: boolean
      interfaceName: string
      stack: string
      mtu: number
      addressIpv4: string
      addressIpv6: string
      autoRoute: boolean
      autoRedirect: boolean
      strictRoute: boolean
    }
    outboundGroups: Array<{ tag: string; type: string; filterMode: string; filter: string; include: string; url: string; interval: string }>
    route: {
      final: string
      defaultDomainResolver: string
      defaultHttpClient: string
      autoDetectInterface: boolean
      findProcess: boolean
      defaultMark: number
      rules: Array<{ type: string; value: string; values?: string[]; action?: string; outbound: string; mode?: string; matchOnly?: boolean; subRules?: Array<{ type: string; value: string }> }>
      ruleSets: Array<{ tag: string; type: string; format: string; url: string; path?: string }>
    }
    experimental: {
      cacheFileEnabled: boolean
      storeFakeip: boolean
      storeRdrc: boolean
      externalController: string
      externalUi: string
      externalUiDownloadUrl: string
      externalUiHttpClient: string
      defaultMode: string
      urltestUnifiedDelay: boolean
    }
    httpClients: Array<{ tag: string; version: number; headers?: Record<string, string>; detour?: string }>
    services: Array<{ type: string; listen: string; listenPort: number }>
    providers: Array<{ type: string; tag: string; url: string; path: string; httpClient: string; updateInterval: string }>
  }
}

// 响应式数据
const mihomoConfigs = ref<MihomoConfig[]>([])
const singboxConfigs = ref<SingBoxConfig[]>([])

// 模态框引用
const mihomoModalRef = ref<HTMLElement | null>(null)
const singboxModalRef = ref<HTMLElement | null>(null)
let mihomoModal: Modal | null = null
let singboxModal: Modal | null = null

// 编辑子模态框引用
const proxyGroupEditorRef = ref<HTMLElement | null>(null)
const ruleProviderEditorRef = ref<HTMLElement | null>(null)
const mihomoRuleEditorRef = ref<HTMLElement | null>(null)
const dnsServerEditorRef = ref<HTMLElement | null>(null)
const dnsRuleEditorRef = ref<HTMLElement | null>(null)
const singboxGroupEditorRef = ref<HTMLElement | null>(null)
const singboxRouteRuleEditorRef = ref<HTMLElement | null>(null)
const singboxRuleSetEditorRef = ref<HTMLElement | null>(null)
const hostEntryEditorRef = ref<HTMLElement | null>(null)

let proxyGroupEditorModal: Modal | null = null
let ruleProviderEditorModal: Modal | null = null
let mihomoRuleEditorModal: Modal | null = null
let dnsServerEditorModal: Modal | null = null
let dnsRuleEditorModal: Modal | null = null
let singboxGroupEditorModal: Modal | null = null
let singboxRouteRuleEditorModal: Modal | null = null
let singboxRuleSetEditorModal: Modal | null = null
let hostEntryEditorModal: Modal | null = null

// 当前编辑的子项
const editingProxyGroupIndex = ref(-1)
const editingRuleProviderIndex = ref(-1)
const editingMihomoRuleIndex = ref(-1)
const editingDnsServerIndex = ref(-1)
const editingDnsRuleIndex = ref(-1)
const editingSingBoxGroupIndex = ref(-1)
const editingSingBoxRouteRuleIndex = ref(-1)
const editingSingBoxRuleSetIndex = ref(-1)

// 域名条目编辑状态
const editingHostEntryDomain = ref<string | null>(null)  // null=添加, string=编辑原域名
const editingHostEntryDomainInput = ref('')
const editingHostEntryIps = ref('')

// 编辑项目的计算引用
const editingProxyGroup = ref<any>(null)
const editingRuleProvider = ref<any>(null)
const editingMihomoRule = ref<any>(null)
const editingDnsServer = ref<any>(null)
const editingDnsRule = ref<any>(null)
const editingSingBoxGroup = ref<any>(null)
const editingSingBoxRouteRule = ref<any>(null)
const editingSingBoxRuleSet = ref<any>(null)

// 默认 Mihomo 配置 (与 sublink.go 保持一致)
const getDefaultMihomoConfig = (): MihomoConfig => ({
  id: 0,
  name: '',
  description: '',
  enabled: false,
  modules: ['基础设置', 'Sniffer', 'TUN', 'DNS', '策略组', '规则'],
  config: {
    // 基础设置
    mixedPort: 7891,
    redirPort: 9797,
    tproxyPort: 9898,
    mode: 'rule',
    bindAddress: '*',
    logLevel: 'error',
    ipv6: true,
    allowLan: true,
    unifiedDelay: false,
    tcpConcurrent: true,
    externalController: '0.0.0.0:9090',
    externalUi: './dashboard',
    externalUiUrl: '',
    findProcessMode: 'always',
    globalClientFingerprint: 'random',
    profile: {
      storeSelected: true,
      storeFakeip: true
    },
    // Sniffer
    sniffer: {
      enable: true,
      sniff: {
        HTTP: { ports: '80, 8080-8880', overrideDestination: true },
        TLS: { ports: '443, 8443' },
        QUIC: { ports: '443, 8443' }
      },
      skipDomain: ''
    },
    // TUN
    tun: {
      enable: true,
      device: 'meta',
      stack: 'gvisor',
      dnsHijack: 'any:53\ntcp://any:53',
      udpTimeout: 300,
      autoRoute: true,
      autoRedirect: false,
      autoDetectInterface: true,
      strictRoute: true
    },
    // DNS
    dns: {
      enable: true,
      ipv6: true,
      listen: '0.0.0.0:1053',
      enhancedMode: 'fake-ip',
      fakeIpRange: '198.18.0.1/16',
      defaultNameserver: '',
      nameserver: '8.8.8.8',
      proxyServerNameserver: '223.5.5.5',
      nameserverPolicy: 'rule-set:cn_domain: 223.5.5.5'
    },
    // 策略组 (动态生成，这里仅为模板示例)
    proxyGroups: [
      { name: '国外', type: 'select', filterMode: 'geoip-cn', filter: '!cn', excludeFilter: '', includeAll: false, url: '', interval: 0, timeout: 0, tolerance: 0, lazy: true, hidden: false, strategy: '' },
      { name: '国内', type: 'select', filterMode: 'geoip-cn', filter: 'cn', excludeFilter: '', includeAll: false, url: '', interval: 0, timeout: 0, tolerance: 0, lazy: true, hidden: false, strategy: '' },
      { name: 'AI', type: 'select', filterMode: 'geoip-country', filter: 'US,JP,SG,KR,TW,GB,DE', excludeFilter: '', includeAll: false, url: '', interval: 0, timeout: 0, tolerance: 0, lazy: true, hidden: false, strategy: '' },
      { name: 'wificall', type: 'select', filterMode: 'geoip-country', filter: 'US,GB,DE', excludeFilter: '', includeAll: false, url: '', interval: 0, timeout: 0, tolerance: 0, lazy: true, hidden: false, strategy: '' }
    ],
    // 规则集 (与 sublink.go rule-providers 一致)
    ruleProviders: [
      { name: 'AI', type: 'http', behavior: 'domain', format: 'mrs', url: 'https://ghfast.top/github.com/QuixoticHeart/rule-set/raw/refs/heads/ruleset/meta/domain/ai.mrs', interval: 86400, proxy: 'DIRECT' },
      { name: 'cn_domain', type: 'http', behavior: 'domain', format: 'mrs', url: 'https://ghfast.top/github.com/QuixoticHeart/rule-set/raw/refs/heads/ruleset/meta/domain/cn.mrs', interval: 86400, proxy: 'DIRECT' },
      { name: 'cn_ip', type: 'http', behavior: 'ipcidr', format: 'mrs', url: 'https://ghfast.top/github.com/QuixoticHeart/rule-set/raw/refs/heads/ruleset/meta/ipcidr/cn.mrs', interval: 86400, proxy: 'DIRECT' },
      { name: 'private_domain', type: 'http', behavior: 'domain', format: 'mrs', url: 'https://ghfast.top/github.com/QuixoticHeart/rule-set/raw/refs/heads/ruleset/meta/domain/private.mrs', interval: 86400, proxy: 'DIRECT' },
      { name: 'private_ip', type: 'http', behavior: 'ipcidr', format: 'mrs', url: 'https://ghfast.top/github.com/QuixoticHeart/rule-set/raw/refs/heads/ruleset/meta/ipcidr/private.mrs', interval: 86400, proxy: 'DIRECT' },
      { name: 'proxy_domain', type: 'http', behavior: 'domain', format: 'mrs', url: 'https://ghfast.top/github.com/QuixoticHeart/rule-set/raw/refs/heads/ruleset/meta/domain/proxy.mrs', interval: 86400, proxy: 'DIRECT' }
    ],
    // 路由规则 (与 sublink.go rules 一致)
    rules: [
      
      { type: 'OR', value: '', outbound: 'DIRECT', noResolve: false, subRules: [
        { type: 'RULE-SET', value: 'private_ip' },
        { type: 'RULE-SET', value: 'private_domain' }
      ]},
      { type: 'OR', value: '', outbound: 'wificall', noResolve: false, subRules: [
        { type: 'DOMAIN-SUFFIX', value: 'ls.apple.com' },
        { type: 'DOMAIN-SUFFIX', value: '3gppnetwork.org' }
      ]},
      { type: 'DST-PORT', value: '500/4500', outbound: 'wificall', noResolve: false },
      { type: 'DOMAIN-KEYWORD', value: 'stun', outbound: 'REJECT', noResolve: false },
      { type: 'RULE-SET', value: 'AI', outbound: 'AI', noResolve: false },
      { type: 'RULE-SET', value: 'proxy_domain', outbound: '国外', noResolve: false },
      { type: 'OR', value: '', outbound: '国内', noResolve: false, subRules: [
        { type: 'RULE-SET', value: 'cn_ip' },
        { type: 'RULE-SET', value: 'cn_domain' }
      ]},
      { type: 'MATCH', value: '', outbound: '国外', noResolve: false }
    ]
  }
})

// 默认 SingBox 配置 (与 sublink.go 保持一致，兼容 sing-box 1.14)
const getDefaultSingBoxConfig = (): SingBoxConfig => ({
  id: 0,
  name: '',
  description: '',
  enabled: false,
  modules: ['Log', 'DNS', 'Inbound', 'Outbound', 'Route', 'Experimental', 'NTP', 'HttpClients', 'Services'],
  config: {
    log: {
      disabled: false,
      level: 'warn',
      output: 'sing-box.log',
      timestamp: true,
    },
    dns: {
      servers: [
        { tag: 'dns-hosts', type: 'hosts', server: '', serverPort: 0, detour: '', domainResolver: '', inet4Range: '', inet6Range: '', predefined: { 'dns.alidns.com': ['223.5.5.5', '223.6.6.6', '2400:3200::1', '2400:3200:baba::1'], 'dns.google': ['8.8.8.8', '8.8.4.4', '2001:4860:4860::8888', '2001:4860:4860::8844'] } },
        { tag: 'bootstrap', type: 'udp', server: '223.5.5.5', serverPort: 0, detour: '国内流量', domainResolver: 'dns-hosts', inet4Range: '', inet6Range: '' },
        { tag: 'aliyun', type: 'quic', server: 'dns.alidns.com', serverPort: 0, detour: '国内流量', domainResolver: 'dns-hosts', inet4Range: '', inet6Range: '' },
        { tag: 'cloudflare', type: 'https', server: 'cloudflare-dns.com', serverPort: 0, detour: '国外流量', domainResolver: 'dns-hosts', inet4Range: '', inet6Range: '' },
        { tag: 'google', type: 'https', server: 'dns.google', serverPort: 0, detour: '国外流量', domainResolver: 'dns-hosts', inet4Range: '', inet6Range: '' },
        { tag: 'CN DNS', type: 'group', server: '', serverPort: 0, detour: '', domainResolver: '', inet4Range: '', inet6Range: '', servers: ['aliyun', 'bootstrap'] },
        { tag: 'PROXY DNS', type: 'group', server: '', serverPort: 0, detour: '', domainResolver: '', inet4Range: '', inet6Range: '', servers: ['cloudflare', 'google'] },
        { tag: 'fakeip-resolver', type: 'fakeip', server: '', serverPort: 0, detour: '', domainResolver: '', inet4Range: '198.18.0.0/15', inet6Range: '' },
      ],
      strategy: 'prefer_ipv4',
      final: 'PROXY DNS',
      clientSubnet: '',
      optimistic: true,
      reverseMapping: false,
      disableCache: false,
      cacheCapacity: 8192,
      rules: [
        { type: 'clash_mode', value: 'direct', values: [], server: 'CN DNS', action: '', rewriteTtl: false },
        { type: 'clash_mode', value: 'global', values: [], server: 'PROXY DNS', action: '', rewriteTtl: false },
        { type: 'rule_set', value: '', values: ['cn_domain', 'private_domain'], server: 'CN DNS', action: '', rewriteTtl: false },
        { type: 'query_type', value: 'A,AAAA', values: [], server: 'fakeip-resolver', action: '', rewriteTtl: true },
      ]
    },
    ntp: {
      enabled: true,
      interval: '1h0m0s',
      server: 'ntp.aliyun.com',
      serverPort: 123,
    },
    inbound: {
      tunEnable: true,
      interfaceName: 'sing',
      stack: 'mixed',
      mtu: 9000,
      addressIpv4: '172.18.0.1/30',
      addressIpv6: 'fdfe:dcba:9876::1/126',
      autoRoute: true,
      autoRedirect: true,
      strictRoute: true
    },
    outboundGroups: [
      { tag: '国外流量', type: 'selector', filterMode: '', filter: '', include: '(?i)-', url: '', interval: '' },
      { tag: '国内流量', type: 'selector', filterMode: 'geoip-cn', filter: 'cn', include: '', url: '', interval: '' },
      { tag: 'AI 服务', type: 'selector', filterMode: 'geoip-country', filter: 'US,JP,SG,KR,TW,GB,DE', include: '(?i)-', url: '', interval: '' },
      { tag: 'wificall', type: 'selector', filterMode: 'geoip-country', filter: 'US,GB,DE', include: '(?i)-', url: '', interval: '' },
    ],
    route: {
      final: '国外流量',
      defaultDomainResolver: 'CN DNS',
      defaultHttpClient: 'sources_downloader',
      autoDetectInterface: true,
      findProcess: true,
      defaultMark: 255,
      rules: [
        { type: 'logical', value: '', values: [], action: 'hijack-dns', outbound: '', mode: 'or', subRules: [
          { type: 'protocol', value: 'dns' },
          { type: 'port', value: '53' }
        ]},
        { type: 'clash_mode', value: 'direct', values: [], action: '', outbound: 'DIRECT', mode: '', subRules: [] },
        { type: 'clash_mode', value: 'global', values: [], action: '', outbound: '国外流量', mode: '', subRules: [] },
        { type: 'domain_suffix', value: 'ls.apple.com,3gppnetwork.org', values: [], action: '', outbound: 'wificall', mode: '', subRules: [] },
        { type: 'port', value: '500,4500', values: [], action: '', outbound: 'wificall', mode: '', subRules: [] },
        { type: 'rule_set', value: '', values: ['AI'], action: '', outbound: 'AI 服务', mode: '', subRules: [] },
        { type: 'rule_set', value: '', values: ['google_domain', 'geolocation-!cn'], action: '', outbound: '国外流量', mode: '', subRules: [] },
        { type: 'rule_set', value: '', values: ['cn_domain'], action: '', outbound: '国内流量', mode: '', subRules: [] },
        { type: '', value: '', values: [], action: 'resolve', outbound: '', mode: '', matchOnly: true, subRules: [] },
        { type: 'rule_set', value: '', values: ['google_ip'], action: '', outbound: '国外流量', mode: '', subRules: [] },
        { type: 'rule_set', value: '', values: ['cn_ip'], action: '', outbound: '国内流量', mode: '', subRules: [] },
        { type: 'ip_is_private', value: '', values: [], action: '', outbound: 'DIRECT', mode: '', subRules: [] },
      ],
      ruleSets: [
        { tag: 'private_domain', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geosite/private.srs' },
        { tag: 'cn_domain', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geosite/cn.srs' },
        { tag: 'google_domain', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geosite/google.srs' },
        { tag: 'geolocation-!cn', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geosite/geolocation-!cn.srs' },
        { tag: 'AI', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://github.com/DustinWin/ruleset_geodata/releases/download/sing-box-ruleset/ai.srs' },
        { tag: 'private_ip', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geoip/private.srs' },
        { tag: 'cn_ip', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geoip/cn.srs' },
        { tag: 'google_ip', type: 'remote', format: '', url: 'https://v6.gh-proxy.org/https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/refs/heads/sing/geo/geoip/google.srs' },
      ]
    },
    experimental: {
      cacheFileEnabled: true,
      storeFakeip: false,
      storeRdrc: true,
      externalController: '0.0.0.0:9090',
      externalUi: 'dashboard',
      externalUiDownloadUrl: 'https://ghfast.top/https://github.com/Zephyruso/zashboard/releases/latest/download/dist.zip',
      externalUiHttpClient: 'sources_downloader',
      defaultMode: 'rule',
      urltestUnifiedDelay: true,
    },
    httpClients: [
      { tag: 'proxy_downloader', version: 2, headers: { 'User-Agent': 'sing-box/1.14' }, detour: '国外流量' },
      { tag: 'sources_downloader', version: 2, headers: { 'User-Agent': 'Mozilla/5.0 (Linux; Android 16; Pixel 10 Pro Build/UPB5.240623.009) AppleWebKit/537.36' }, detour: '国外流量' },
    ],
    services: [
      { type: 'api', listen: '127.0.0.1', listenPort: 9091 }
    ],
    providers: [
      { type: 'remote', tag: '自建', url: '', path: './proxy_provider/custom.json', httpClient: 'proxy_downloader', updateInterval: '24h0m0s' }
    ]
  }
})

// 当前编辑的配置
const editingMihomo = reactive<MihomoConfig>(getDefaultMihomoConfig())
const editingSingBox = reactive<SingBoxConfig>(getDefaultSingBoxConfig())

// 加载状态
const loading = ref(false)
const saving = ref(false)

// 初始化
onMounted(async () => {
  if (mihomoModalRef.value) {
    mihomoModal = new Modal(mihomoModalRef.value)
  }
  if (singboxModalRef.value) {
    singboxModal = new Modal(singboxModalRef.value)
  }
  
  // 从 API 加载配置
  await loadConfigs()
})

// 从 API 加载配置
const loadConfigs = async () => {
  loading.value = true
  let needSave = false
  
  try {
    const response = await getSubscriptionConfigs()
    const data = response.data
    
    if (data.mihomoConfigs && data.mihomoConfigs.length > 0) {
      mihomoConfigs.value = data.mihomoConfigs
    } else {
      // 如果没有配置，使用默认配置
      mihomoConfigs.value = [
        {
          ...getDefaultMihomoConfig(),
          id: 1,
          name: '默认配置',
          description: '标准 Mihomo 配置模板',
          enabled: true
        }
      ]
      needSave = true
    }
    
    if (data.singboxConfigs && data.singboxConfigs.length > 0) {
      singboxConfigs.value = data.singboxConfigs
    } else {
      // 如果没有配置，使用默认配置
      singboxConfigs.value = [
        {
          ...getDefaultSingBoxConfig(),
          id: 1,
          name: '默认配置',
          description: '标准 SingBox 配置模板',
          enabled: true
        }
      ]
      needSave = true
    }
    
    // 如果创建了默认配置，自动保存到服务器
    if (needSave) {
      await saveSubscriptionConfigs({
        mihomoConfigs: mihomoConfigs.value,
        singboxConfigs: singboxConfigs.value
      })
      console.log('默认配置已自动保存到服务器')
    }
  } catch (error) {
    console.error('加载订阅配置失败:', error)
    // 加载失败时使用默认配置
    mihomoConfigs.value = [
      {
        ...getDefaultMihomoConfig(),
        id: 1,
        name: '默认配置',
        description: '标准 Mihomo 配置模板',
        enabled: true
      }
    ]
    singboxConfigs.value = [
      {
        ...getDefaultSingBoxConfig(),
        id: 1,
        name: '默认配置',
        description: '标准 SingBox 配置模板',
        enabled: true
      }
    ]
  } finally {
    loading.value = false
  }
}

// 保存所有配置到服务器
const saveAllConfigs = async () => {
  saving.value = true
  try {
    await saveSubscriptionConfigs({
      mihomoConfigs: mihomoConfigs.value,
      singboxConfigs: singboxConfigs.value
    })
  } catch (error) {
    console.error('保存订阅配置失败:', error)
    alert('保存失败，请重试')
  } finally {
    saving.value = false
  }
}

// 初始化 accordion - Bootstrap 已通过 data-bs-* 属性自动处理
const initAccordion = (_accordionId: string) => {
  // Bootstrap 5 的 accordion 通过 data-bs-toggle="collapse" 自动工作
  // 不需要手动初始化 Collapse 实例
}

// Mihomo 配置操作
const addMihomoConfig = () => {
  Object.assign(editingMihomo, getDefaultMihomoConfig())
  mihomoModal?.show()
  initAccordion('mihomoAccordion')
}

const editMihomoConfig = (config: MihomoConfig) => {
  Object.assign(editingMihomo, JSON.parse(JSON.stringify(config)))
  mihomoModal?.show()
  initAccordion('mihomoAccordion')
}

const deleteMihomoConfig = async (config: MihomoConfig) => {
  if (confirm(`确定要删除配置 "${config.name}" 吗？`)) {
    const index = mihomoConfigs.value.findIndex(c => c.id === config.id)
    if (index !== -1) {
      mihomoConfigs.value.splice(index, 1)
      await saveAllConfigs()
    }
  }
}

const saveMihomoConfig = async () => {
  if (!editingMihomo.name) {
    alert('请输入配置名称')
    return
  }
  
  if (editingMihomo.id) {
    // 更新
    const index = mihomoConfigs.value.findIndex(c => c.id === editingMihomo.id)
    if (index !== -1) {
      mihomoConfigs.value[index] = JSON.parse(JSON.stringify(editingMihomo))
    }
  } else {
    // 新增
    const newConfig = JSON.parse(JSON.stringify(editingMihomo))
    newConfig.id = Date.now()
    mihomoConfigs.value.push(newConfig)
  }
  
  mihomoModal?.hide()
  await saveAllConfigs()
}

const onMihomoToggle = async (config: MihomoConfig) => {
  if (config.enabled) {
    // 禁用其他配置（只能有一个启用）
    mihomoConfigs.value.forEach(c => {
      if (c.id !== config.id) {
        c.enabled = false
      }
    })
  }
  await saveAllConfigs()
}

// 策略组操作
const addProxyGroup = () => {
  editingMihomo.config.proxyGroups.push({
    name: '',
    type: 'select',
    filterMode: 'regex',
    filter: '',
    excludeFilter: '',
    includeAll: true,
    url: 'http://www.gstatic.com/generate_204',
    interval: 300,
    timeout: 5000,
    tolerance: 50,
    lazy: true,
    hidden: false,
    strategy: ''
  })
  // 自动打开新添加的编辑器
  openProxyGroupEditor(editingMihomo.config.proxyGroups.length - 1)
}

const openProxyGroupEditor = (index: number) => {
  editingProxyGroupIndex.value = index
  editingProxyGroup.value = editingMihomo.config.proxyGroups[index]
  if (!proxyGroupEditorModal && proxyGroupEditorRef.value) {
    proxyGroupEditorModal = new Modal(proxyGroupEditorRef.value, { backdrop: 'static' })
  }
  proxyGroupEditorModal?.show()
}

const removeProxyGroup = (index: number) => {
  editingMihomo.config.proxyGroups.splice(index, 1)
}

// 规则集操作
const addRuleProvider = () => {
  editingMihomo.config.ruleProviders.push({ name: '', type: 'http', behavior: 'domain', format: 'mrs', url: '', interval: 86400, proxy: 'DIRECT' })
  openRuleProviderEditor(editingMihomo.config.ruleProviders.length - 1)
}

const openRuleProviderEditor = (index: number) => {
  editingRuleProviderIndex.value = index
  editingRuleProvider.value = editingMihomo.config.ruleProviders[index]
  if (!ruleProviderEditorModal && ruleProviderEditorRef.value) {
    ruleProviderEditorModal = new Modal(ruleProviderEditorRef.value, { backdrop: 'static' })
  }
  ruleProviderEditorModal?.show()
}

const removeRuleProvider = (index: number) => {
  editingMihomo.config.ruleProviders.splice(index, 1)
}

// Mihomo 路由规则操作
const addMihomoRule = () => {
  editingMihomo.config.rules.push({ type: 'RULE-SET', value: '', outbound: '', noResolve: false, subRules: [] })
  openMihomoRuleEditor(editingMihomo.config.rules.length - 1)
}

const openMihomoRuleEditor = (index: number) => {
  editingMihomoRuleIndex.value = index
  editingMihomoRule.value = editingMihomo.config.rules[index]
  if (!editingMihomoRule.value.subRules) {
    editingMihomoRule.value.subRules = []
  }
  if (!mihomoRuleEditorModal && mihomoRuleEditorRef.value) {
    mihomoRuleEditorModal = new Modal(mihomoRuleEditorRef.value, { backdrop: 'static' })
  }
  mihomoRuleEditorModal?.show()
}

const onEditingMihomoRuleTypeChange = () => {
  if (editingMihomoRule.value && ['AND', 'OR', 'NOT'].includes(editingMihomoRule.value.type)) {
    if (!editingMihomoRule.value.subRules) {
      editingMihomoRule.value.subRules = []
    }
    editingMihomoRule.value.value = ''
  }
}

const removeMihomoRule = (index: number) => {
  editingMihomo.config.rules.splice(index, 1)
}

// 格式化Mihomo规则显示
const formatMihomoRule = (rule: any) => {
  if (['AND', 'OR', 'NOT'].includes(rule.type)) {
    return `${rule.type}(${rule.subRules?.length || 0}子规则),${rule.outbound || '?'}`
  }
  if (rule.type === 'MATCH') {
    return `MATCH,${rule.outbound || '?'}`
  }
  return `${rule.type},${rule.value || '?'},${rule.outbound || '?'}`
}

// 获取 Mihomo 规则占位符文本
const getMihomoRulePlaceholder = (type: string): string => {
  const placeholders: Record<string, string> = {
    'DOMAIN': 'example.com',
    'DOMAIN-SUFFIX': 'google.com',
    'DOMAIN-KEYWORD': 'google',
    'DOMAIN-REGEX': '^abc.*com',
    'GEOSITE': 'cn,google',
    'GEOIP': 'CN,US',
    'IP-CIDR': '192.168.0.0/16',
    'IP-ASN': '13335',
    'SRC-IP-CIDR': '192.168.1.0/24',
    'DST-PORT': '80,443,8080-8880',
    'SRC-PORT': '7777',
    'PROCESS-NAME': 'chrome.exe',
    'PROCESS-PATH': '/usr/bin/wget',
    'NETWORK': 'tcp/udp'
  }
  return placeholders[type] || 'value'
}

// SingBox 配置操作
const addSingBoxConfig = () => {
  Object.assign(editingSingBox, getDefaultSingBoxConfig())
  singboxModal?.show()
  initAccordion('singboxAccordion')
}

const editSingBoxConfig = (config: SingBoxConfig) => {
  Object.assign(editingSingBox, JSON.parse(JSON.stringify(config)))
  singboxModal?.show()
  initAccordion('singboxAccordion')
}

const deleteSingBoxConfig = async (config: SingBoxConfig) => {
  if (confirm(`确定要删除配置 "${config.name}" 吗？`)) {
    const index = singboxConfigs.value.findIndex(c => c.id === config.id)
    if (index !== -1) {
      singboxConfigs.value.splice(index, 1)
      await saveAllConfigs()
    }
  }
}

const saveSingBoxConfig = async () => {
  if (!editingSingBox.name) {
    alert('请输入配置名称')
    return
  }
  
  if (editingSingBox.id) {
    // 更新
    const index = singboxConfigs.value.findIndex(c => c.id === editingSingBox.id)
    if (index !== -1) {
      singboxConfigs.value[index] = JSON.parse(JSON.stringify(editingSingBox))
    }
  } else {
    // 新增
    const newConfig = JSON.parse(JSON.stringify(editingSingBox))
    newConfig.id = Date.now()
    singboxConfigs.value.push(newConfig)
  }
  
  singboxModal?.hide()
  await saveAllConfigs()
}

const onSingBoxToggle = async (config: SingBoxConfig) => {
  if (config.enabled) {
    // 禁用其他配置（只能有一个启用）
    singboxConfigs.value.forEach(c => {
      if (c.id !== config.id) {
        c.enabled = false
      }
    })
  }
  await saveAllConfigs()
}

// SingBox 策略组操作
const addSingBoxGroup = () => {
  editingSingBox.config.outboundGroups.push({ tag: '', type: 'selector', filterMode: 'regex', filter: '', url: 'http://www.gstatic.com/generate_204', interval: '3m' })
  openSingBoxGroupEditor(editingSingBox.config.outboundGroups.length - 1)
}

const openSingBoxGroupEditor = (index: number) => {
  editingSingBoxGroupIndex.value = index
  editingSingBoxGroup.value = editingSingBox.config.outboundGroups[index]
  if (!singboxGroupEditorModal && singboxGroupEditorRef.value) {
    singboxGroupEditorModal = new Modal(singboxGroupEditorRef.value, { backdrop: 'static' })
  }
  singboxGroupEditorModal?.show()
}

const removeSingBoxGroup = (index: number) => {
  editingSingBox.config.outboundGroups.splice(index, 1)
}

// SingBox DNS 服务器操作
const addSingBoxDnsServer = () => {
  editingSingBox.config.dns.servers.push({ tag: '', type: 'udp', server: '', serverPort: 53, detour: '', domainResolver: '', inet4Range: '', inet6Range: '', predefined: {} as Record<string, string[]> })
  openDnsServerEditor(editingSingBox.config.dns.servers.length - 1)
}

const openDnsServerEditor = (index: number) => {
  editingDnsServerIndex.value = index
  const srv = editingSingBox.config.dns.servers[index]
  if (!srv.predefined) {
    srv.predefined = {}
  }
  editingDnsServer.value = srv
  if (!dnsServerEditorModal && dnsServerEditorRef.value) {
    dnsServerEditorModal = new Modal(dnsServerEditorRef.value, { backdrop: 'static' })
  }
  dnsServerEditorModal?.show()
}

// 域名映射条目编辑
const openHostEntryEditor = (domain: string) => {
  const srv = editingDnsServer.value
  if (!srv) return
  if (domain) {
    // 编辑已有条目
    editingHostEntryDomain.value = domain
    editingHostEntryDomainInput.value = domain
    editingHostEntryIps.value = (srv.predefined[domain] || []).join(', ')
  } else {
    // 添加新条目
    editingHostEntryDomain.value = null
    editingHostEntryDomainInput.value = ''
    editingHostEntryIps.value = ''
  }
  if (!hostEntryEditorModal && hostEntryEditorRef.value) {
    hostEntryEditorModal = new Modal(hostEntryEditorRef.value, { backdrop: 'static' })
  }
  hostEntryEditorModal?.show()
}

const saveHostEntry = () => {
  const srv = editingDnsServer.value
  if (!srv) return
  const newDomain = editingHostEntryDomainInput.value.trim()
  if (!newDomain) return

  if (!srv.predefined) srv.predefined = {}

  // 如果是编辑且域名变了，删掉旧的 key
  if (editingHostEntryDomain.value !== null && editingHostEntryDomain.value !== newDomain) {
    delete srv.predefined[editingHostEntryDomain.value]
  }

  // 分割 IP
  const ips = editingHostEntryIps.value.split(',').map(s => s.trim()).filter(Boolean)
  srv.predefined[newDomain] = ips

  hostEntryEditorModal?.hide()
}

const removeHostEntry = (domain: string) => {
  const srv = editingDnsServer.value
  if (!srv || !srv.predefined) return
  delete srv.predefined[domain]
}

const removeSingBoxDnsServer = (index: number) => {
  editingSingBox.config.dns.servers.splice(index, 1)
}

// SingBox 规则集操作
const addSingBoxRuleSet = () => {
  editingSingBox.config.route.ruleSets.push({ tag: '', type: 'remote', format: 'binary', url: '' })
  openSingBoxRuleSetEditor(editingSingBox.config.route.ruleSets.length - 1)
}

const openSingBoxRuleSetEditor = (index: number) => {
  editingSingBoxRuleSetIndex.value = index
  editingSingBoxRuleSet.value = editingSingBox.config.route.ruleSets[index]
  if (!singboxRuleSetEditorModal && singboxRuleSetEditorRef.value) {
    singboxRuleSetEditorModal = new Modal(singboxRuleSetEditorRef.value, { backdrop: 'static' })
  }
  singboxRuleSetEditorModal?.show()
}

// SingBox DNS 规则操作
const addSingBoxDnsRule = () => {
  editingSingBox.config.dns.rules.push({ type: 'rule_set', value: '', values: [], server: '', action: '', rewriteTtl: false })
  openDnsRuleEditor(editingSingBox.config.dns.rules.length - 1)
}

const openDnsRuleEditor = (index: number) => {
  editingDnsRuleIndex.value = index
  editingDnsRule.value = editingSingBox.config.dns.rules[index]
  if (!editingDnsRule.value.values) {
    editingDnsRule.value.values = []
  }
  if (!dnsRuleEditorModal && dnsRuleEditorRef.value) {
    dnsRuleEditorModal = new Modal(dnsRuleEditorRef.value, { backdrop: 'static' })
  }
  dnsRuleEditorModal?.show()
}

const toggleEditingDnsRuleSet = (tag: string) => {
  if (!editingDnsRule.value.values) editingDnsRule.value.values = []
  const idx = editingDnsRule.value.values.indexOf(tag)
  if (idx === -1) {
    editingDnsRule.value.values.push(tag)
  } else {
    editingDnsRule.value.values.splice(idx, 1)
  }
}

const removeSingBoxDnsRule = (index: number) => {
  editingSingBox.config.dns.rules.splice(index, 1)
}

// 拖拽排序相关
const dragState = reactive({
  dragging: false,
  dragIndex: -1,
  dropIndex: -1,
  listType: ''
})

const onDragStart = (event: DragEvent, index: number, listType: string) => {
  dragState.dragging = true
  dragState.dragIndex = index
  dragState.listType = listType
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', index.toString())
  }
  // 添加拖拽样式
  const target = event.target as HTMLElement
  target.classList.add('dragging')
}

const onDragOver = (event: DragEvent, index: number, listType: string) => {
  if (dragState.listType !== listType) return
  event.preventDefault()
  dragState.dropIndex = index
  
  // 添加视觉提示
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const y = event.clientY - rect.top
  if (y < rect.height / 2) {
    target.classList.add('drag-over-top')
    target.classList.remove('drag-over-bottom')
  } else {
    target.classList.add('drag-over-bottom')
    target.classList.remove('drag-over-top')
  }
}

const onDrop = (event: DragEvent, listType: string) => {
  event.preventDefault()
  if (dragState.listType !== listType || dragState.dragIndex === -1) return
  
  const fromIndex = dragState.dragIndex
  const toIndex = dragState.dropIndex
  
  if (fromIndex === toIndex) return
  
  // 获取对应的数组
  let arr: any[] | null = null
  switch (listType) {
    case 'mihomoRuleProviders':
      arr = editingMihomo.config.ruleProviders
      break
    case 'mihomoRules':
      arr = editingMihomo.config.rules
      break
    case 'singboxDnsServers':
      arr = editingSingBox.config.dns.servers
      break
    case 'singboxDnsRules':
      arr = editingSingBox.config.dns.rules
      break
    case 'singboxRouteRules':
      arr = editingSingBox.config.route.rules
      break
    case 'singboxRuleSets':
      arr = editingSingBox.config.route.ruleSets
      break
  }
  
  if (arr) {
    const [item] = arr.splice(fromIndex, 1)
    arr.splice(toIndex, 0, item)
  }
  
  // 清理状态
  cleanupDragState()
}

const onDragEnd = () => {
  cleanupDragState()
  // 移除所有拖拽样式
  document.querySelectorAll('.dragging, .drag-over-top, .drag-over-bottom').forEach(el => {
    el.classList.remove('dragging', 'drag-over-top', 'drag-over-bottom')
  })
}

const cleanupDragState = () => {
  dragState.dragging = false
  dragState.dragIndex = -1
  dragState.dropIndex = -1
  dragState.listType = ''
}

// SingBox 路由规则操作
const addSingBoxRouteRule = () => {
  editingSingBox.config.route.rules.push({ type: 'rule_set', value: '', values: [], action: '', outbound: '', subRules: [] })
  openSingBoxRouteRuleEditor(editingSingBox.config.route.rules.length - 1)
}

const openSingBoxRouteRuleEditor = (index: number) => {
  editingSingBoxRouteRuleIndex.value = index
  editingSingBoxRouteRule.value = editingSingBox.config.route.rules[index]
  if (!editingSingBoxRouteRule.value.values) {
    editingSingBoxRouteRule.value.values = []
  }
  if (!editingSingBoxRouteRule.value.subRules) {
    editingSingBoxRouteRule.value.subRules = []
  }
  if (!singboxRouteRuleEditorModal && singboxRouteRuleEditorRef.value) {
    singboxRouteRuleEditorModal = new Modal(singboxRouteRuleEditorRef.value, { backdrop: 'static' })
  }
  singboxRouteRuleEditorModal?.show()
}

const toggleEditingRouteRuleSet = (tag: string) => {
  if (!editingSingBoxRouteRule.value.values) editingSingBoxRouteRule.value.values = []
  const idx = editingSingBoxRouteRule.value.values.indexOf(tag)
  if (idx === -1) {
    editingSingBoxRouteRule.value.values.push(tag)
  } else {
    editingSingBoxRouteRule.value.values.splice(idx, 1)
  }
}

const onEditingSingBoxRouteRuleTypeChange = () => {
  if (editingSingBoxRouteRule.value && editingSingBoxRouteRule.value.type === 'logical') {
    if (!editingSingBoxRouteRule.value.subRules) {
      editingSingBoxRouteRule.value.subRules = []
    }
    editingSingBoxRouteRule.value.mode = editingSingBoxRouteRule.value.mode || 'or'
    editingSingBoxRouteRule.value.value = ''
  }
}

// 格式化SingBox路由规则显示
const formatSingBoxRouteRule = (rule: any) => {
  if (rule.type === 'logical') {
    return `${rule.mode || 'or'}(${rule.subRules?.length || 0})`
  }
  if (rule.type === 'rule_set') {
    return rule.values?.join(', ') || ''
  }
  if (rule.type === 'ip_is_private') {
    return 'private'
  }
  return rule.value || ''
}

const removeSingBoxRouteRule = (index: number) => {
  editingSingBox.config.route.rules.splice(index, 1)
}

// 获取子规则占位符文本
const getSubRulePlaceholder = (type: string): string => {
  const placeholders: Record<string, string> = {
    'domain': 'example.com (comma separated)',
    'domain_suffix': '.example.com (comma separated)',
    'domain_keyword': 'keyword (comma separated)',
    'domain_regex': 'regex pattern',
    'ip_cidr': '192.168.0.0/24 (comma separated)',
    'port': '80,443',
    'protocol': 'dns,tls',
    'network': 'tcp,udp',
    'rule_set': 'select rule_set'
  }
  return placeholders[type] || 'value'
}

const removeSingBoxRuleSet = (index: number) => {
  editingSingBox.config.route.ruleSets.splice(index, 1)
}
</script>

<style scoped>
.card {
  transition: all 0.2s ease;
}

.card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.border-primary {
  border-width: 2px !important;
}

.accordion-button:not(.collapsed) {
  background-color: rgba(102, 126, 234, 0.1);
}

.font-monospace {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 0.875rem;
}

.badge {
  font-weight: normal;
}

/* 规则集标签选择器样式 */
.rule-set-selector .selected-tags .badge {
  font-size: 0.75rem;
  padding: 0.25em 0.5em;
}

.rule-set-selector .selected-tags .badge i {
  font-size: 0.875rem;
  opacity: 0.8;
}

.rule-set-selector .selected-tags .badge i:hover {
  opacity: 1;
}

.rule-set-selector .dropdown-menu .dropdown-item {
  font-size: 0.875rem;
  padding: 0.375rem 0.75rem;
}

.rule-set-selector .dropdown-menu .dropdown-item:hover {
  background-color: rgba(0, 123, 255, 0.1);
}

.rule-set-selector .dropdown-toggle {
  text-align: left;
  font-size: 0.875rem;
}

.rule-set-selector .dropdown-toggle::after {
  position: absolute;
  right: 0.5rem;
  top: 50%;
  transform: translateY(-50%);
}

/* 下拉框内的选中标签样式 */
.selected-tags-inline .badge {
  font-size: 0.7rem;
  padding: 0.15em 0.4em;
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.selected-tags-inline .badge i {
  font-size: 0.75rem;
  opacity: 0.8;
}

.selected-tags-inline .badge i:hover {
  opacity: 1;
}

/* 拖拽排序样式 */
.draggable-item {
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.draggable-item:hover .drag-handle {
  color: #6c757d !important;
}

.draggable-item.dragging {
  opacity: 0.5;
  transform: scale(0.98);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.draggable-item.drag-over-top {
  border-top: 2px solid #0d6efd !important;
}

.draggable-item.drag-over-bottom {
  border-bottom: 2px solid #0d6efd !important;
}

.drag-handle {
  font-size: 1.1rem;
  padding: 0 0.25rem;
  transition: color 0.15s ease;
}

.drag-handle:active {
  cursor: grabbing !important;
}

/* 卡片式列表样式 */
.item-card {
  padding: 10px 14px;
  border: 1px solid #dee2e6;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s ease;
}

.item-card:hover {
  border-color: #0d6efd;
  background: #f8f9ff;
}

.item-card .item-name {
  font-weight: 500;
  white-space: nowrap;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-card .item-detail {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 0.875rem;
}
</style>
