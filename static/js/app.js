class ImageProcessor {
    constructor() {
        this.apiBaseUrl = '/api/images';
        this.pollingInterval = 5000; // 5 seconds
        this.isUploading = false;
        this.lastUploadTime = 0;
        this.pollingIntervalId = null;
        this.processedImages = new Map(); // Для отслеживания уже обработанных изображений
        this.isPolling = false;
        this.initEventListeners();
        this.loadImages();
        this.startPolling();
    }

    initEventListeners() {
        const uploadForm = document.getElementById('uploadForm');
        const fileInput = document.getElementById('fileInput');
        const watermarkCheckbox = document.getElementById('watermark');
        
        // Удаляем старые обработчики если они есть
        uploadForm.removeEventListener('submit', this.handleUpload.bind(this));
        fileInput.removeEventListener('change', this.handleFileSelect.bind(this));
        watermarkCheckbox.removeEventListener('change', this.toggleWatermarkText.bind(this));
        
        // Добавляем новые обработчики с правильным контекстом
        uploadForm.addEventListener('submit', (e) => {
            e.preventDefault();
            this.handleUpload(e);
        }, { once: false });
        
        fileInput.addEventListener('change', (e) => this.handleFileSelect(e));
        watermarkCheckbox.addEventListener('change', (e) => this.toggleWatermarkText(e));
        
        console.log('Event listeners initialized');
    }

    async handleUpload(e) {
        console.log('handleUpload called');
        
        // Защита от двойной отправки
        if (this.isUploading) {
            console.log('Предотвращена двойная отправка: уже загружается');
            return;
        }
        
        const now = Date.now();
        if (now - this.lastUploadTime < 2000) {
            console.log('Предотвращена частая отправка: прошло менее 2 секунд');
            this.showAlert('Пожалуйста, подождите 2 секунды перед следующей загрузкой', 'warning');
            return;
        }
        
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
        
        // Проверяем, не загружали ли уже этот файл (по имени и размеру)
        const fileKey = `${file.name}_${file.size}_${file.lastModified}`;
        if (this.processedImages.has(fileKey)) {
            console.log('Файл уже был загружен ранее:', fileKey);
            this.showAlert('Этот файл уже был загружен', 'info');
            return;
        }
        
        this.isUploading = true;
        this.lastUploadTime = now;
        
        this.setUploadingState(true);
        
        const formData = new FormData();
        formData.append('file', file);
        formData.append('thumbnail', document.getElementById('thumbnail').checked);
        formData.append('resize', document.getElementById('resize').checked);
        formData.append('watermark', document.getElementById('watermark').checked);
        
        const watermarkText = document.getElementById('watermarkText').value;
        if (watermarkText) {
            formData.append('watermark_text', watermarkText);
        }
        
        try {
            console.log('Отправка запроса на загрузку...');
            const response = await fetch(`${this.apiBaseUrl}/upload`, {
                method: 'POST',
                body: formData
            });
            
            console.log('Ответ получен, статус:', response.status);
            
            if (!response.ok) {
                const error = await response.json().catch(() => ({ message: 'Ошибка загрузки' }));
                throw new Error(error.message || 'Ошибка загрузки');
            }
            
            const result = await response.json();
            console.log('Успешный ответ:', result);
            
            this.showAlert('Изображение загружено успешно! Обработка начата.', 'success');
            
            // Добавляем файл в обработанные
            this.processedImages.set(fileKey, {
                id: result.id,
                timestamp: Date.now()
            });
            
            // Добавляем изображение в список
            this.addImageToList(result);
            
            // Очищаем форму
            fileInput.value = '';
            document.getElementById('preview').style.display = 'none';
            document.getElementById('fileName').textContent = '';
            
        } catch (error) {
            console.error('Upload error:', error);
            this.showAlert(error.message || 'Ошибка при загрузке изображения', 'danger');
        } finally {
            this.setUploadingState(false);
            this.isUploading = false;
            
            // Через 5 минут удаляем из processedImages чтобы можно было загрузить тот же файл снова
            setTimeout(() => {
                this.processedImages.delete(fileKey);
            }, 5 * 60 * 1000); // 5 минут
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
            console.log('Загрузка списка изображений...');
            // В реальном приложении здесь был бы запрос к API для получения списка
            const imageList = document.getElementById('imageList');
            
            // Проверяем, есть ли уже изображения
            const items = imageList.querySelectorAll('.image-item');
            if (items.length === 0) {
                // Показываем только один раз при загрузке
                if (!imageList.dataset.initialized) {
                    imageList.innerHTML = '<p class="text-center text-muted">Загруженные изображения появятся здесь</p>';
                    imageList.dataset.initialized = 'true';
                }
            }
        } catch (error) {
            console.error('Ошибка загрузки изображений:', error);
        }
    }

    startPolling() {
        // Останавливаем предыдущий интервал если он был
        if (this.pollingIntervalId) {
            clearInterval(this.pollingIntervalId);
        }
        
        console.log('Запуск polling с интервалом:', this.pollingInterval);
        this.pollingIntervalId = setInterval(() => {
            if (!this.isPolling) {
                this.checkProcessingStatus();
            }
        }, this.pollingInterval);
    }

    async checkProcessingStatus() {
        if (this.isPolling) {
            console.log('Предотвращен одновременный polling');
            return;
        }
        
        this.isPolling = true;
        
        try {
            const processingItems = document.querySelectorAll('.image-item[data-status="processing"], .image-item[data-status="uploaded"]');
            
            if (processingItems.length > 0) {
                console.log('Проверка статуса для', processingItems.length, 'изображений');
                
                // Используем Promise.allSettled для параллельной проверки
                const promises = Array.from(processingItems).map(async (item) => {
                    const imageId = item.dataset.id;
                    try {
                        const response = await fetch(`${this.apiBaseUrl}/${imageId}/status`, {
                            headers: {
                                'Cache-Control': 'no-cache',
                                'Pragma': 'no-cache'
                            }
                        });
                        
                        if (response.ok) {
                            const statusData = await response.json();
                            // Обновляем только если статус изменился
                            if (item.dataset.status !== statusData.status) {
                                console.log('Обновление статуса для', imageId, 'с', item.dataset.status, 'на', statusData.status);
                                this.updateImageStatus(imageId, statusData.status);
                            }
                        }
                    } catch (error) {
                        console.error('Ошибка проверки статуса для', imageId, ':', error);
                    }
                });
                
                await Promise.allSettled(promises);
            }
        } catch (error) {
            console.error('Ошибка в checkProcessingStatus:', error);
        } finally {
            this.isPolling = false;
        }
    }

    addImageToList(image) {
        const imageList = document.getElementById('imageList');
        
        // Удаляем сообщение о пустом списке
        const emptyMessage = imageList.querySelector('p');
        if (emptyMessage && emptyMessage.textContent.includes('Загруженные изображения появятся здесь')) {
            emptyMessage.remove();
        }
        
        // Проверяем, нет ли уже этого изображения в списке
        const existingItem = imageList.querySelector(`.image-item[data-id="${image.id}"]`);
        if (existingItem) {
            console.log('Изображение уже есть в списке, обновляем статус:', image.id);
            this.updateImageStatus(image.id, image.status);
            return;
        }
        
        console.log('Добавление нового изображения в список:', image.id);
        
        const item = document.createElement('div');
        item.className = `image-item status-${image.status}`;
        item.dataset.id = image.id;
        item.dataset.status = image.status;
        
        const statusText = this.getStatusText(image.status);
        const formattedDate = new Date(image.created_at).toLocaleString('ru-RU');
        
        item.innerHTML = `
            <div class="image-info">
                <div class="image-name" title="${image.filename}">${image.filename}</div>
                <div class="image-meta">
                    <div class="image-size">${this.formatFileSize(image.size)}</div>
                    <div class="image-status status-${image.status}">${statusText}</div>
                    <div class="image-date">${formattedDate}</div>
                </div>
            </div>
            <div class="image-actions">
                ${image.status === 'completed' ? `
                    <button class="btn btn-sm btn-view" onclick="window.imageProcessor.viewImage('${image.id}', 'thumbnail')" title="Просмотр миниатюры">
                        <i class="fas fa-th"></i>
                    </button>
                    <button class="btn btn-sm btn-view" onclick="window.imageProcessor.viewImage('${image.id}', 'resize')" title="Просмотр ресайза">
                        <i class="fas fa-expand"></i>
                    </button>
                    <button class="btn btn-sm btn-view" onclick="window.imageProcessor.viewImage('${image.id}', '')" title="Просмотр оригинала">
                        <i class="fas fa-eye"></i>
                    </button>
                ` : ''}
                <button class="btn btn-sm btn-delete" onclick="window.imageProcessor.deleteImage('${image.id}')" title="Удалить">
                    <i class="fas fa-trash"></i>
                </button>
            </div>
        `;
        
        // Добавляем в начало списка
        imageList.prepend(item);
    }

    updateImageStatus(imageId, status) {
        const item = document.querySelector(`.image-item[data-id="${imageId}"]`);
        if (item) {
            const oldStatus = item.dataset.status;
            item.dataset.status = status;
            item.className = `image-item status-${status}`;
            
            const statusElement = item.querySelector('.image-status');
            if (statusElement) {
                statusElement.className = `image-status status-${status}`;
                statusElement.textContent = this.getStatusText(status);
            }
            
            // Если статус изменился на "completed", обновляем кнопки
            if (status === 'completed' && oldStatus !== 'completed') {
                const actionsElement = item.querySelector('.image-actions');
                if (actionsElement) {
                    actionsElement.innerHTML = `
                        <button class="btn btn-sm btn-view" onclick="window.imageProcessor.viewImage('${imageId}', 'thumbnail')" title="Просмотр миниатюры">
                            <i class="fas fa-th"></i>
                        </button>
                        <button class="btn btn-sm btn-view" onclick="window.imageProcessor.viewImage('${imageId}', 'resize')" title="Просмотр ресайза">
                            <i class="fas fa-expand"></i>
                        </button>
                        <button class="btn btn-sm btn-view" onclick="window.imageProcessor.viewImage('${imageId}', '')" title="Просмотр оригинала">
                            <i class="fas fa-eye"></i>
                        </button>
                        <button class="btn btn-sm btn-delete" onclick="window.imageProcessor.deleteImage('${imageId}')" title="Удалить">
                            <i class="fas fa-trash"></i>
                        </button>
                    `;
                }
                
                // Показываем сообщение только один раз при завершении обработки
                if (!item.dataset.notified) {
                    this.showAlert(`Обработка изображения "${item.querySelector('.image-name').textContent}" завершена!`, 'success');
                    item.dataset.notified = 'true';
                }
            }
        }
    }

    async viewImage(imageId, operation = '') {
        try {
            let url = `${this.apiBaseUrl}/${imageId}`;
            if (operation) {
                url += `?operation=${operation}`;
            }
            
            // Открываем в модальном окне
            const modalElement = document.getElementById('imageModal');
            if (!modalElement) {
                console.error('Модальное окно не найдено');
                return;
            }
            
            const modal = new bootstrap.Modal(modalElement);
            const modalImage = document.getElementById('modalImage');
            const imageInfo = document.getElementById('imageInfo');
            const downloadLink = document.getElementById('downloadLink');
            
            if (!modalImage || !imageInfo || !downloadLink) {
                console.error('Элементы модального окна не найдены');
                return;
            }
            
            modalImage.src = url;
            downloadLink.href = url;
            downloadLink.download = `image_${imageId}_${operation || 'original'}.jpg`;
            
            // Получаем информацию об изображении
            try {
                const response = await fetch(`${this.apiBaseUrl}/${imageId}/status`);
                if (response.ok) {
                    const data = await response.json();
                    imageInfo.innerHTML = `
                        <div>ID: ${imageId}</div>
                        <div>Статус: ${this.getStatusText(data.status)}</div>
                        <div>Версия: ${operation ? operation : 'оригинал'}</div>
                    `;
                }
            } catch (error) {
                console.error('Ошибка при получении информации об изображении:', error);
                imageInfo.innerHTML = `
                    <div>ID: ${imageId}</div>
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
                    
                    // Удаляем из отслеживаемых processedImages
                    const fileName = item.querySelector('.image-name').textContent;
                    
                    // Находим и удаляем соответствующий ключ из processedImages
                    for (const [key, value] of this.processedImages.entries()) {
                        if (value.id === imageId) {
                            this.processedImages.delete(key);
                            console.log('Удален из processedImages:', key);
                            break;
                        }
                    }
                }
                
                this.showAlert('Изображение удалено успешно', 'success');
                
                // Показываем сообщение о пустом списке если нужно
                this.checkEmptyList();
                
            } else {
                const error = await response.json();
                throw new Error(error.message || 'Ошибка удаления');
            }
        } catch (error) {
            console.error('Delete error:', error);
            this.showAlert(error.message, 'danger');
        }
    }

    checkEmptyList() {
        const imageList = document.getElementById('imageList');
        const items = imageList.querySelectorAll('.image-item');
        
        if (items.length === 0 && !imageList.querySelector('p')) {
            imageList.innerHTML = '<p class="text-center text-muted">Загруженные изображения появятся здесь</p>';
        }
    }

    getStatusText(status) {
        const statusMap = {
            'uploaded': 'Загружено',
            'processing': 'В\u00A0обработке', // Используем неразрывный пробел
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
        
        if (uploadBtn) {
            uploadBtn.disabled = isUploading;
            uploadBtn.innerHTML = isUploading 
                ? '<i class="fas fa-spinner fa-spin"></i> Загрузка...' 
                : '<i class="fas fa-upload"></i> Загрузить и обработать';
        }
        
        if (uploadSpinner) {
            uploadSpinner.style.display = isUploading ? 'block' : 'none';
        }
    }

    showAlert(message, type) {
        const alertsContainer = document.getElementById('alerts');
        if (!alertsContainer) return;
        
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
        
        // Автоматическое удаление через 5 секунд
        setTimeout(() => {
            const alert = document.getElementById(alertId);
            if (alert) {
                alert.remove();
            }
        }, 5000);
    }

    // Очистка ресурсов при уничтожении
    destroy() {
        console.log('Очистка ресурсов ImageProcessor');
        
        if (this.pollingIntervalId) {
            clearInterval(this.pollingIntervalId);
            this.pollingIntervalId = null;
        }
        
        // Удаляем все обработчики событий
        const uploadForm = document.getElementById('uploadForm');
        const fileInput = document.getElementById('fileInput');
        const watermarkCheckbox = document.getElementById('watermark');
        
        if (uploadForm) {
            uploadForm.removeEventListener('submit', this.handleUpload.bind(this));
        }
        if (fileInput) {
            fileInput.removeEventListener('change', this.handleFileSelect.bind(this));
        }
        if (watermarkCheckbox) {
            watermarkCheckbox.removeEventListener('change', this.toggleWatermarkText.bind(this));
        }
    }
}

// Инициализация приложения
let imageProcessor = null;

document.addEventListener('DOMContentLoaded', () => {
    console.log('DOM загружен, инициализация ImageProcessor...');
    
    // Очищаем предыдущий экземпляр если есть
    if (imageProcessor) {
        console.log('Очистка предыдущего экземпляра ImageProcessor');
        imageProcessor.destroy();
    }
    
    // Создаем новый экземпляр
    try {
        imageProcessor = new ImageProcessor();
        console.log('ImageProcessor успешно инициализирован');
    } catch (error) {
        console.error('Ошибка инициализации ImageProcessor:', error);
        return;
    }
    
    // Делаем глобально доступным
    window.imageProcessor = imageProcessor;
    
    console.log('ImageProcessor готов к работе');
});

// Очистка при выгрузке страницы
window.addEventListener('beforeunload', () => {
    console.log('Очистка ImageProcessor перед выгрузкой страницы');
    if (imageProcessor) {
        imageProcessor.destroy();
    }
});