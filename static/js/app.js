class ImageProcessor {
    constructor() {
        this.apiBaseUrl = '/api/images';
        this.pollingInterval = 5000; // 5 seconds
        this.initEventListeners();
        this.loadImages();
        this.startPolling();
    }

    initEventListeners() {
        const uploadForm = document.getElementById('uploadForm');
        const fileInput = document.getElementById('fileInput');
        const watermarkCheckbox = document.getElementById('watermark');
        
        uploadForm.addEventListener('submit', (e) => this.handleUpload(e));
        fileInput.addEventListener('change', (e) => this.handleFileSelect(e));
        watermarkCheckbox.addEventListener('change', (e) => this.toggleWatermarkText(e));
    }

    async handleUpload(e) {
        e.preventDefault();
        
        const fileInput = document.getElementById('fileInput');
        const file = fileInput.files[0];
        
        if (!file) {
            this.showAlert('Пожалуйста, выберите файл', 'warning');
            return;
        }
        
        if (!file.type.startsWith('image/')) {
            this.showAlert('Пожалуйста, выберите изображение', 'warning');
            return;
        }
        
        if (file.size > 32 * 1024 * 1024) {
            this.showAlert('Файл слишком большой. Максимальный размер: 32MB', 'warning');
            return;
        }
        
        const formData = new FormData();
        formData.append('file', file);
        formData.append('thumbnail', document.getElementById('thumbnail').checked);
        formData.append('resize', document.getElementById('resize').checked);
        formData.append('watermark', document.getElementById('watermark').checked);
        
        const watermarkText = document.getElementById('watermarkText').value;
        if (watermarkText) {
            formData.append('watermark_text', watermarkText);
        }
        
        this.setUploadingState(true);
        
        try {
            const response = await fetch(`${this.apiBaseUrl}/upload`, {
                method: 'POST',
                body: formData
            });
            
            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.message || 'Ошибка загрузки');
            }
            
            const result = await response.json();
            this.showAlert('Изображение загружено успешно! Обработка начата.', 'success');
            this.addImageToList(result);
            fileInput.value = '';
            document.getElementById('preview').style.display = 'none';
            document.getElementById('fileName').textContent = '';
        } catch (error) {
            console.error('Upload error:', error);
            this.showAlert(error.message || 'Ошибка при загрузке изображения', 'danger');
        } finally {
            this.setUploadingState(false);
        }
    }

    handleFileSelect(e) {
        const file = e.target.files[0];
        if (file) {
            const fileName = document.getElementById('fileName');
            fileName.textContent = `Выбран файл: ${file.name} (${this.formatFileSize(file.size)})`;
            fileName.style.color = '#28a745';
            
            // Preview
            const preview = document.getElementById('preview');
            const reader = new FileReader();
            
            reader.onload = function(e) {
                preview.src = e.target.result;
                preview.style.display = 'block';
            }
            
            reader.readAsDataURL(file);
        }
    }

    toggleWatermarkText(e) {
        const watermarkTextGroup = document.getElementById('watermarkTextGroup');
        watermarkTextGroup.style.display = e.target.checked ? 'block' : 'none';
    }

    async loadImages() {
        try {
            // In a real app, you would have an endpoint for listing images
            // For now, we'll just show a message
            const imageList = document.getElementById('imageList');
            
            // Check if there are any images in the list
            const items = imageList.querySelectorAll('.image-item');
            if (items.length === 0) {
                imageList.innerHTML = '<p class="text-center text-muted">Загруженные изображения появятся здесь</p>';
            }
        } catch (error) {
            console.error('Ошибка загрузки изображений:', error);
        }
    }

    startPolling() {
        // Poll for status updates
        setInterval(() => this.checkProcessingStatus(), this.pollingInterval);
    }

    async checkProcessingStatus() {
        const processingItems = document.querySelectorAll('.image-item[data-status="processing"], .image-item[data-status="uploaded"]');
        
        for (const item of processingItems) {
            const imageId = item.dataset.id;
            try {
                const response = await fetch(`${this.apiBaseUrl}/${imageId}/status`);
                if (response.ok) {
                    const statusData = await response.json();
                    this.updateImageStatus(imageId, statusData.status);
                }
            } catch (error) {
                console.error('Ошибка проверки статуса:', error);
            }
        }
    }

    addImageToList(image) {
        const imageList = document.getElementById('imageList');
        
        // Remove default message
        if (imageList.querySelector('p')) {
            imageList.innerHTML = '';
        }
        
        const item = document.createElement('div');
        item.className = `image-item status-${image.status}`;
        item.dataset.id = image.id;
        item.dataset.status = image.status;
        
        const statusText = this.getStatusText(image.status);
        const formattedDate = new Date(image.created_at).toLocaleString('ru-RU');
        
        item.innerHTML = `
            <div class="image-info">
                <div class="image-name" title="${image.filename}">${image.filename}</div>
                <div class="image-size">${this.formatFileSize(image.size)}</div>
                <div class="image-status status-${image.status}">${statusText}</div>
                <div class="image-date">${formattedDate}</div>
            </div>
            <div class="image-actions">
                ${image.status === 'completed' ? `
                    <button class="btn btn-sm btn-view" onclick="imageProcessor.viewImage('${image.id}', 'thumbnail')" title="Просмотр миниатюры">
                        <i class="fas fa-th"></i>
                    </button>
                    <button class="btn btn-sm btn-view" onclick="imageProcessor.viewImage('${image.id}', 'resize')" title="Просмотр ресайза">
                        <i class="fas fa-expand"></i>
                    </button>
                    <button class="btn btn-sm btn-view" onclick="imageProcessor.viewImage('${image.id}', '')" title="Просмотр оригинала">
                        <i class="fas fa-eye"></i>
                    </button>
                ` : ''}
                <button class="btn btn-sm btn-delete" onclick="imageProcessor.deleteImage('${image.id}')" title="Удалить">
                    <i class="fas fa-trash"></i>
                </button>
            </div>
        `;
        
        imageList.prepend(item);
    }

    updateImageStatus(imageId, status) {
        const item = document.querySelector(`.image-item[data-id="${imageId}"]`);
        if (item) {
            item.dataset.status = status;
            item.className = `image-item status-${status}`;
            
            const statusElement = item.querySelector('.image-status');
            if (statusElement) {
                statusElement.className = `image-status status-${status}`;
                statusElement.textContent = this.getStatusText(status);
            }
            
            // Add view buttons if processing is complete
            if (status === 'completed') {
                const actionsElement = item.querySelector('.image-actions');
                actionsElement.innerHTML = `
                    <button class="btn btn-sm btn-view" onclick="imageProcessor.viewImage('${imageId}', 'thumbnail')" title="Просмотр миниатюры">
                        <i class="fas fa-th"></i>
                    </button>
                    <button class="btn btn-sm btn-view" onclick="imageProcessor.viewImage('${imageId}', 'resize')" title="Просмотр ресайза">
                        <i class="fas fa-expand"></i>
                    </button>
                    <button class="btn btn-sm btn-view" onclick="imageProcessor.viewImage('${imageId}', '')" title="Просмотр оригинала">
                        <i class="fas fa-eye"></i>
                    </button>
                    <button class="btn btn-sm btn-delete" onclick="imageProcessor.deleteImage('${imageId}')" title="Удалить">
                        <i class="fas fa-trash"></i>
                    </button>
                `;
            }
            
            // Show success message when processing completes
            if (status === 'completed' && item.dataset.status !== 'completed') {
                this.showAlert('Обработка изображения завершена!', 'success');
            }
        }
    }

    async viewImage(imageId, operation = '') {
        try {
            let url = `${this.apiBaseUrl}/${imageId}`;
            if (operation) {
                url += `?operation=${operation}`;
            }
            
            // Open in modal
            const modal = new bootstrap.Modal(document.getElementById('imageModal'));
            const modalImage = document.getElementById('modalImage');
            const imageInfo = document.getElementById('imageInfo');
            const downloadLink = document.getElementById('downloadLink');
            
            modalImage.src = url;
            downloadLink.href = url;
            downloadLink.download = `image_${imageId}_${operation || 'original'}.jpg`;
            
            // Get image info
            const response = await fetch(`${this.apiBaseUrl}/${imageId}/status`);
            if (response.ok) {
                const data = await response.json();
                imageInfo.innerHTML = `
                    <div>ID: ${imageId}</div>
                    <div>Статус: ${this.getStatusText(data.status)}</div>
                    <div>Версия: ${operation ? operation : 'оригинал'}</div>
                `;
            }
            
            modal.show();
        } catch (error) {
            console.error('Ошибка при открытии изображения:', error);
            this.showAlert('Ошибка при открытии изображения', 'danger');
        }
    }

    async deleteImage(imageId) {
        if (!confirm('Вы уверены, что хотите удалить это изображение и все его обработанные версии?')) {
            return;
        }
        
        try {
            const response = await fetch(`${this.apiBaseUrl}/${imageId}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                const item = document.querySelector(`.image-item[data-id="${imageId}"]`);
                if (item) {
                    item.remove();
                }
                this.showAlert('Изображение удалено успешно', 'success');
                
                // Reload images list
                this.loadImages();
            } else {
                const error = await response.json();
                throw new Error(error.message || 'Ошибка удаления');
            }
        } catch (error) {
            console.error('Delete error:', error);
            this.showAlert(error.message, 'danger');
        }
    }

    getStatusText(status) {
        const statusMap = {
            'uploaded': 'Загружено',
            'processing': 'В обработке',
            'completed': 'Готово',
            'failed': 'Ошибка',
            'deleted': 'Удалено'
        };
        return statusMap[status] || status;
    }

    formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    setUploadingState(isUploading) {
        const uploadBtn = document.getElementById('uploadBtn');
        const uploadSpinner = document.getElementById('uploadSpinner');
        
        uploadBtn.disabled = isUploading;
        uploadSpinner.style.display = isUploading ? 'block' : 'none';
        
        if (isUploading) {
            uploadBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Загрузка...';
            uploadBtn.classList.add('disabled');
        } else {
            uploadBtn.innerHTML = '<i class="fas fa-upload"></i> Загрузить и обработать';
            uploadBtn.classList.remove('disabled');
        }
    }

    showAlert(message, type) {
        const alertsContainer = document.getElementById('alerts');
        const alertId = 'alert-' + Date.now();
        
        const alertDiv = document.createElement('div');
        alertDiv.id = alertId;
        alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
        alertDiv.role = 'alert';
        alertDiv.innerHTML = `
            ${message}
            <button type="button" class="btn-close" onclick="document.getElementById('${alertId}').remove()"></button>
        `;
        
        alertsContainer.prepend(alertDiv);
        
        // Auto-remove after 5 seconds
        setTimeout(() => {
            const alert = document.getElementById(alertId);
            if (alert) {
                alert.remove();
            }
        }, 5000);
    }
}

// Initialize the application
let imageProcessor;

document.addEventListener('DOMContentLoaded', () => {
    imageProcessor = new ImageProcessor();
});

// Make functions available globally for onclick handlers
window.imageProcessor = imageProcessor;