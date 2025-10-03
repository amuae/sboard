/**
 * 简洁的删除确认弹框组件
 * 符合项目 Bootstrap 5 + 渐变背景的设计风格
 */

console.log('简洁确认弹框脚本已加载');

window.showDeleteConfirm = function(options) {
    console.log('showDeleteConfirm 被调用:', options);
    
    const {
        title = '确认删除',
        itemName = '',
        onConfirm = null,
        onCancel = null
    } = options;
    
    // 如果提供了项目名称，显示确认问题
    const displayTitle = itemName ? `确认要删除 ${itemName} 吗？` : title;
    
    // 创建简洁美观的模态框HTML
    const modalHTML = `
        <div class="modal fade" id="beautifulConfirmModal" tabindex="-1" data-bs-backdrop="static" data-bs-keyboard="false">
            <div class="modal-dialog modal-dialog-centered modal-sm">
                <div class="modal-content border-0 shadow-lg" style="border-radius: 16px; overflow: hidden;">
                    <!-- 渐变顶部装饰 -->
                    <div style="height: 4px; background: linear-gradient(90deg, #667eea, #764ba2);"></div>
                    
                    <div class="modal-header border-0 pb-3 pt-4" style="background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);">
                        <div class="w-100 text-center">
                            <h6 class="modal-title mb-2 fw-bold text-dark" style="font-size: 1.1rem;">${displayTitle}</h6>
                            <small class="text-muted">此操作不可撤销</small>
                        </div>
                    </div>
                    
                    <div class="modal-footer border-0 pt-0 px-4 pb-4">
                        <div class="d-flex gap-3 w-100">
                            <button type="button" class="btn btn-light flex-fill" 
                                    id="beautifulCancelBtn"
                                    style="
                                        border-radius: 25px; 
                                        border: 2px solid #dee2e6;
                                        transition: all 0.3s ease;
                                        font-weight: 500;
                                    "
                                    onmouseover="this.style.transform='translateY(-2px)'; this.style.boxShadow='0 4px 12px rgba(0,0,0,0.15)';"
                                    onmouseout="this.style.transform='translateY(0)'; this.style.boxShadow='none';">
                                取消
                            </button>
                            <button type="button" class="btn btn-danger flex-fill" 
                                    id="beautifulConfirmBtn"
                                    style="
                                        border-radius: 25px;
                                        background: linear-gradient(135deg, #e74c3c, #c0392b);
                                        border: none;
                                        transition: all 0.3s ease;
                                        font-weight: 500;
                                        box-shadow: 0 4px 15px rgba(231, 76, 60, 0.3);
                                    "
                                    onmouseover="this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(231, 76, 60, 0.4)';"
                                    onmouseout="this.style.transform='translateY(0)'; this.style.boxShadow='0 4px 15px rgba(231, 76, 60, 0.3)';">
                                确认删除
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        
        <style>
        /* 自定义动画 */
        @keyframes modalSlideIn {
            from {
                opacity: 0;
                transform: scale(0.7) translateY(-20px);
            }
            to {
                opacity: 1;
                transform: scale(1) translateY(0);
            }
        }
        
        #beautifulConfirmModal .modal-content {
            animation: modalSlideIn 0.3s ease-out;
        }
        
        /* 按钮点击效果 */
        #beautifulConfirmBtn:active {
            transform: translateY(-1px) scale(0.98) !important;
        }
        
        #beautifulCancelBtn:active {
            transform: translateY(-1px) scale(0.98) !important;
        }
        </style>
    `;
    
    // 移除之前的模态框
    const existingModal = document.getElementById('beautifulConfirmModal');
    if (existingModal) {
        existingModal.remove();
    }
    
    // 添加新的模态框
    document.body.insertAdjacentHTML('beforeend', modalHTML);
    
    // 获取按钮元素
    const confirmBtn = document.getElementById('beautifulConfirmBtn');
    const cancelBtn = document.getElementById('beautifulCancelBtn');
    
    // 绑定取消按钮事件
    cancelBtn.addEventListener('click', function() {
        const modal = bootstrap.Modal.getInstance(document.getElementById('beautifulConfirmModal'));
        modal.hide();
        if (onCancel) onCancel();
    });
    
    // 绑定确认按钮事件
    confirmBtn.addEventListener('click', function() {
        if (onConfirm) {
            // 显示加载状态
            confirmBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>删除中...';
            confirmBtn.disabled = true;
            cancelBtn.disabled = true;
            
            // 执行删除操作
            onConfirm(() => {
                // 删除完成后关闭弹框
                const modal = bootstrap.Modal.getInstance(document.getElementById('beautifulConfirmModal'));
                if (modal) {
                    modal.hide();
                }
            });
        }
    });
    
    // 监听模态框关闭事件，清理DOM
    document.getElementById('beautifulConfirmModal').addEventListener('hidden.bs.modal', function() {
        this.remove();
    });
    
    // 显示模态框
    const modal = new bootstrap.Modal(document.getElementById('beautifulConfirmModal'));
    modal.show();
    
    console.log('简洁模态框已显示');
};

// 其他类型的确认框
window.showWarningConfirm = function(options) {
    return window.showDeleteConfirm({
        ...options,
        title: options.title || '警告确认'
    });
};

window.showInfoConfirm = function(options) {
    return window.showDeleteConfirm({
        ...options,
        title: options.title || '信息确认'
    });
};

console.log('简洁确认弹框函数已定义');
