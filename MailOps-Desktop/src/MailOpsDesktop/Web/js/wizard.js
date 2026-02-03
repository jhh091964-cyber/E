class MailOpsWizard {
    constructor() {
        this.currentStep = 1;
        this.totalSteps = 6;
        this.data = {
            token: '',
            domain: '',
            vpsIp: ''
        };
        
        this.initializeElements();
        this.bindEvents();
    }

    initializeElements() {
        // Buttons
        this.btnPrev = document.getElementById('btnPrev');
        this.btnNext = document.getElementById('btnNext');
        
        // Step 1 inputs
        this.inputToken = document.getElementById('cloudflareToken');
        this.inputDomain = document.getElementById('domain');
        this.inputVpsIp = document.getElementById('vpsIp');
        this.domainError = document.getElementById('domainError');
        
        // Step 4 checkbox
        this.confirmCheckbox = document.getElementById('confirmCheckbox');
    }

    bindEvents() {
        this.btnPrev.addEventListener('click', () => this.prevStep());
        this.btnNext.addEventListener('click', () => this.nextStep());
        
        // Input validation
        this.inputDomain.addEventListener('input', () => this.validateDomain());
        this.inputVpsIp.addEventListener('input', () => this.validateVpsIp());
        
        // Confirm checkbox
        this.confirmCheckbox.addEventListener('change', () => this.updateNavigationButtons());
    }

    prevStep() {
        if (this.currentStep > 1) {
            this.currentStep--;
            this.updateUI();
        }
    }

    async nextStep() {
        // Validate current step before proceeding
        if (!await this.validateCurrentStep()) {
            return;
        }

        // Execute step logic
        await this.executeStep(this.currentStep);

        if (this.currentStep < this.totalSteps) {
            this.currentStep++;
            this.updateUI();
        }
    }

    updateUI() {
        // Update progress bar
        document.querySelectorAll('.step').forEach((stepEl, index) => {
            const stepNum = index + 1;
            stepEl.classList.remove('active', 'completed');
            
            if (stepNum < this.currentStep) {
                stepEl.classList.add('completed');
            } else if (stepNum === this.currentStep) {
                stepEl.classList.add('active');
            }
        });

        // Update step content
        document.querySelectorAll('.step-content').forEach((contentEl, index) => {
            contentEl.classList.remove('active');
            if (index + 1 === this.currentStep) {
                contentEl.classList.add('active');
            }
        });

        // Update navigation buttons
        this.updateNavigationButtons();
    }

    updateNavigationButtons() {
        this.btnPrev.disabled = this.currentStep === 1;
        
        // Step 4 requires checkbox confirmation
        if (this.currentStep === 4 && !this.confirmCheckbox.checked) {
            this.btnNext.disabled = true;
            this.btnNext.textContent = '請先確認';
        } else {
            this.btnNext.disabled = false;
            this.btnNext.textContent = this.currentStep === this.totalSteps ? '完成' : '下一步';
        }
    }

    async validateCurrentStep() {
        switch (this.currentStep) {
            case 1:
                return await this.validateStep1();
            case 2:
                return await this.validateStep2();
            case 3:
                return await this.validateStep3();
            default:
                return true;
        }
    }

    async validateStep1() {
        // Validate token
        this.data.token = this.inputToken.value.trim();
        if (!this.data.token) {
            alert('請輸入 Cloudflare 授權金鑰');
            return false;
        }

        // Validate domain
        if (!this.validateDomain()) {
            return false;
        }

        // Validate VPS IP
        if (!this.validateVpsIp()) {
            return false;
        }

        this.data.domain = this.inputDomain.value.trim();
        this.data.vpsIp = this.inputVpsIp.value.trim();

        return true;
    }

    validateDomain() {
        const domain = this.inputDomain.value.trim();
        const domainRegex = /^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9](?:\.[a-zA-Z]{2,})+$/;
        
        if (!domain) {
            this.domainError.textContent = '請輸入網域名稱';
            this.inputDomain.style.borderColor = '#dc3545';
            return false;
        }
        
        if (!domainRegex.test(domain)) {
            this.domainError.textContent = '請輸入有效的網域名稱格式（例如：example.com）';
            this.inputDomain.style.borderColor = '#dc3545';
            return false;
        }
        
        this.domainError.textContent = '';
        this.inputDomain.style.borderColor = '#e9ecef';
        return true;
    }

    validateVpsIp() {
        const ip = this.inputVpsIp.value.trim();
        const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/;
        
        if (!ip) {
            alert('請輸入 VPS IP 位址');
            return false;
        }
        
        if (!ipRegex.test(ip)) {
            alert('請輸入有效的 IP 位址格式（例如：45.xxx.xxx.xxx）');
            return false;
        }
        
        // Validate each octet
        const octets = ip.split('.');
        for (let octet of octets) {
            const num = parseInt(octet);
            if (num < 0 || num > 255) {
                alert('IP 位址的每個數字必須在 0-255 之間');
                return false;
            }
        }
        
        return true;
    }

    async validateStep2() {
        // Step 2 validation is handled by the check logic
        return true;
    }

    async validateStep3() {
        // Step 3 validation - ensure DNS preview is loaded
        const dnsPreview = document.getElementById('dnsPreview');
        return !dnsPreview.querySelector('.loading');
    }

    async executeStep(step) {
        switch (step) {
            case 1:
                await this.onStep1Complete();
                break;
            case 2:
                await this.onStep2Execute();
                break;
            case 3:
                await this.onStep3Execute();
                break;
            case 4:
                await this.onStep4Complete();
                break;
            case 5:
                await this.onStep5Execute();
                break;
        }
    }

    async onStep1Complete() {
        // Send token securely to C# backend (in memory only)
        this.sendToCSharp({
            type: 'setToken',
            data: {
                token: this.data.token
            }
        });
        
        // Restart service with new token
        this.sendToCSharp({
            type: 'restartService',
            data: {
                domain: this.data.domain,
                vpsIp: this.data.vpsIp
            }
        });
    }

    async onStep2Execute() {
        // Reset check results
        this.resetCheckResults();
        
        // Start environment check via C#
        this.sendToCSharp({
            type: 'checkEnvironment',
            data: {
                domain: this.data.domain,
                vpsIp: this.data.vpsIp
            }
        });
    }

    resetCheckResults() {
        document.querySelectorAll('.check-item').forEach(item => {
            item.classList.remove('success', 'error', 'checking');
            item.querySelector('.check-icon').textContent = '⏳';
            item.querySelector('.check-status').textContent = '檢查中...';
        });
    }

    updateCheckResult(checkId, success, message) {
        const checkItem = document.getElementById(checkId);
        if (checkItem) {
            checkItem.classList.remove('checking');
            checkItem.classList.add(success ? 'success' : 'error');
            checkItem.querySelector('.check-icon').textContent = success ? '✅' : '❌';
            checkItem.querySelector('.check-status').textContent = message;
        }
    }

    async onStep3Execute() {
        // Request DNS preview via C#
        this.sendToCSharp({
            type: 'loadDnsPreview',
            data: {
                domain: this.data.domain,
                vpsIp: this.data.vpsIp
            }
        });
    }

    displayDnsRecords(preview) {
        const dnsPreview = document.getElementById('dnsPreview');
        
        if (!preview.records || preview.records.length === 0) {
            dnsPreview.innerHTML = '<div style="text-align: center; padding: 40px; color: #6c757d;">沒有需要變更的 DNS 記錄</div>';
            return;
        }
        
        let html = `
            <table class="dns-table">
                <thead>
                    <tr>
                        <th>操作</th>
                        <th>類型</th>
                        <th>名稱</th>
                        <th>值</th>
                    </tr>
                </thead>
                <tbody>
        `;
        
        preview.records.forEach(record => {
            const actionClass = `action-${record.action}`;
            const actionText = record.action === 'create' ? '新增' : 
                             record.action === 'update' ? '修改' : '刪除';
            
            // Truncate long values
            const displayValue = record.value.length > 50 
                ? record.value.substring(0, 50) + '...' 
                : record.value;
            
            html += `
                <tr>
                    <td class="${actionClass}">${actionText}</td>
                    <td>${record.type}</td>
                    <td>${record.name}</td>
                    <td title="${record.value}">${displayValue}</td>
                </tr>
            `;
        });
        
        html += `
                </tbody>
            </table>
        `;
        
        dnsPreview.innerHTML = html;
        
        // Update stats
        document.getElementById('createCount').textContent = preview.create_count || preview.createCount || 0;
        document.getElementById('updateCount').textContent = preview.update_count || preview.updateCount || 0;
        document.getElementById('deleteCount').textContent = preview.delete_count || preview.deleteCount || 0;
        document.getElementById('totalCount').textContent = preview.records.length;
        
        // Show stats panel
        document.getElementById('dnsStats').style.display = 'block';
    }

    async onStep4Complete() {
        // Update confirm page
        document.getElementById('confirmDomain').textContent = this.data.domain;
        document.getElementById('confirmVpsIp').textContent = this.data.vpsIp;
        document.getElementById('confirmCount').textContent = document.getElementById('totalCount').textContent;
    }

    async onStep5Execute() {
        // Execute DNS changes via C#
        this.sendToCSharp({
            type: 'executeDnsChanges',
            data: {
                domain: this.data.domain,
                vpsIp: this.data.vpsIp
            }
        });
    }

    updateExecuteProgress(step, status, message) {
        const stepMap = {
            'preview': 'stepPreview',
            'confirm': 'stepConfirm',
            'validate': 'stepValidate',
            'execute': 'stepExecute'
        };
        
        const stepElement = document.getElementById(stepMap[step]);
        if (stepElement) {
            const icon = stepElement.querySelector('.step-icon');
            const statusText = stepElement.querySelector('.step-status');
            
            if (status === 'completed') {
                icon.textContent = '✅';
                statusText.textContent = '完成';
            } else if (status === 'error') {
                icon.textContent = '❌';
                statusText.textContent = '失敗';
            } else if (status === 'running') {
                icon.textContent = '⏳';
                statusText.textContent = '執行中...';
            }
        }
    }

    updateProgressBar(percent) {
        document.getElementById('progressFill').style.width = `${percent}%`;
        document.getElementById('progressText').textContent = `${percent}%`;
    }

    showResult(success, message = '') {
        if (success) {
            document.getElementById('resultSuccess').style.display = 'block';
            document.getElementById('resultFailure').style.display = 'none';
            document.getElementById('successDomain').textContent = this.data.domain;
            document.getElementById('successTime').textContent = new Date().toLocaleString('zh-TW');
        } else {
            document.getElementById('resultSuccess').style.display = 'none';
            document.getElementById('resultFailure').style.display = 'block';
            document.getElementById('errorMessage').textContent = message;
        }
    }

    sendToCSharp(message) {
        // Send message to C# backend via WebView2
        if (window.chrome && window.chrome.webview) {
            window.chrome.webview.postMessage(message);
        } else {
            console.log('WebView2 not available, message:', message);
        }
    }

    // Receive messages from C#
    receiveMessageFromCSharp(message) {
        console.log('Received from C#:', message);
        
        switch (message.type) {
            case 'environmentCheckResult':
                this.updateCheckResult('checkService', message.serviceSuccess, message.serviceMessage);
                this.updateCheckResult('checkToken', message.tokenSuccess, message.tokenMessage);
                this.updateCheckResult('checkDomain', message.domainSuccess, message.domainMessage);
                
                if (message.serviceSuccess && message.tokenSuccess && message.domainSuccess) {
                    setTimeout(() => {
                        this.currentStep++;
                        this.updateUI();
                    }, 1000);
                }
                break;
                
            case 'dnsPreviewResult':
                if (message.success) {
                    this.displayDnsRecords(message.data);
                } else {
                    const dnsPreview = document.getElementById('dnsPreview');
                    dnsPreview.innerHTML = `
                        <div style="text-align: center; padding: 40px; color: #dc3545;">
                            ❌ 載入失敗：${message.error}
                        </div>
                    `;
                }
                break;
                
            case 'executeProgress':
                this.updateExecuteProgress(message.step, message.status, message.message);
                if (message.percent !== undefined) {
                    this.updateProgressBar(message.percent);
                }
                break;
                
            case 'executeComplete':
                this.currentStep = 6;
                this.updateUI();
                this.showResult(message.success, message.error);
                break;
        }
    }
}

// Initialize wizard when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    window.wizard = new MailOpsWizard();
});