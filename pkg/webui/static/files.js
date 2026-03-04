// PicoClaw WebUI - File Upload/Download Support

let uploadedFiles = []; // Store uploaded file paths for current message

// Initialize file upload functionality
function initFileUpload() {
    const fileInput = document.getElementById('fileInput');
    const uploadBtn = document.getElementById('uploadBtn');
    const fileList = document.getElementById('fileList');
    
    if (!fileInput) return;
    
    // Handle file selection
    fileInput.addEventListener('change', handleFileSelect);
    
    // Handle drag and drop
    const chatInput = document.getElementById('messageInput');
    if (chatInput) {
        chatInput.addEventListener('dragover', handleDragOver);
        chatInput.addEventListener('dragleave', handleDragLeave);
        chatInput.addEventListener('drop', handleDrop);
    }
}

// Handle file selection
async function handleFileSelect(event) {
    const files = event.target.files;
    const status = document.getElementById('filePickerStatus');

    // Обновляем статус до early return
    if (files.length === 0) {
        status.textContent = 'no files selected';
        return;
    } else if (files.length === 1) {
        status.textContent = files[0].name;
    } else {
        status.textContent = `selected files: ${files.length}`;
    }

    for (let file of files) {
        await uploadFile(file);
    }
    
    // Reset input + сброс статуса
    event.target.value = '';
    status.textContent = 'no files selected';
}


// Upload file to server
async function uploadFile(file) {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('session', getCurrentSession());

    try {
        const response = await fetch('/api/files/upload', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            throw new Error(`Upload failed: ${response.statusText}`);
        }

        const result = await response.json();

        // Add to uploaded files list
        uploadedFiles.push(result.file_path);

        // Show file preview
        addFilePreview(result.file_name, result.file_size, result.file_path);

    } catch (error) {
        console.error('File upload error:', error);
        alert(`Failed to upload file: ${error.message}`);
    }
}

// Add file preview to UI
function addFilePreview(fileName, fileSize, filePath) {
    const fileList = document.getElementById('fileList');
    if (!fileList) return;

    const fileItem = document.createElement('div');
    fileItem.className = 'file-preview';
    fileItem.dataset.filePath = filePath;

    const icon = getFileIcon(fileName);
    const sizeStr = formatFileSize(fileSize);

    fileItem.innerHTML = `
        <span class="file-icon">${icon}</span>
        <span class="file-name">${fileName}</span>
        <span class="file-size">${sizeStr}</span>
        <button class="file-remove" onclick="removeFilePreview('${filePath}')">&times;</button>
    `;

    fileList.appendChild(fileItem);
    fileList.style.display = 'flex';
}

// Remove file preview
function removeFilePreview(filePath) {
    uploadedFiles = uploadedFiles.filter(f => f !== filePath);
    
    const fileList = document.getElementById('file-list');
    if (!fileList) return;
    
    const previews = fileList.querySelectorAll('.file-preview');
    previews.forEach(preview => {
        if (preview.dataset.filePath === filePath) {
            preview.remove();
        }
    });
    
    if (uploadedFiles.length === 0) {
        fileList.style.display = 'none';
    }
}

// Clear all file previews
function clearFilePreviews() {
    uploadedFiles = [];
    const fileList = document.getElementById('fileList');
    if (fileList) {
        fileList.innerHTML = '';
        fileList.style.display = 'none';
    }
    const fileInput = document.getElementById('fileInput');
    if (fileInput) {
        fileInput.value = '';
    }
}

// Get file icon based on extension
function getFileIcon(fileName) {
    const ext = fileName.split('.').pop().toLowerCase();
    const icons = {
        'jpg': '🖼️', 'jpeg': '🖼️', 'png': '🖼️', 'gif': '🖼️', 'webp': '🖼️', 'bmp': '🖼️',
        'pdf': '📄', 'doc': '📄', 'docx': '📄',
        'txt': '📝', 'md': '📝',
        'csv': '📊', 'xls': '📊', 'xlsx': '📊',
        'json': '📋', 'xml': '📋', 'yaml': '📋', 'yml': '📋',
        'js': '💻', 'ts': '💻', 'go': '💻', 'py': '💻', 'java': '💻', 'cpp': '💻', 'c': '💻', 'h': '💻',
        'html': '🌐', 'css': '🎨',
        'zip': '📦', 'tar': '📦', 'gz': '📦', 'rar': '📦',
        'mp3': '🎵', 'wav': '🎵', 'ogg': '🎵',
        'mp4': '🎬', 'avi': '🎬', 'mov': '🎬', 'webm': '🎬'
    };
    return icons[ext] || '📎';
}

// Format file size
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

// Handle drag over
function handleDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.add('drag-over');
}

// Handle drag leave
function handleDragLeave(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.remove('drag-over');
}

// Handle drop
function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    e.currentTarget.classList.remove('drag-over');
    
    const files = e.dataTransfer.files;
    if (files.length > 0) {
        for (let file of files) {
            uploadFile(file);
        }
    }
}

// Get current session
function getCurrentSession() {
    return localStorage.getItem('webui_session') || '';
}

// Send message with files
async function sendMessageWithFiles(message) {
    const session = getCurrentSession();
    const files = [...uploadedFiles];
    
    // Clear previews before sending
    clearFilePreviews();
    
    try {
        const response = await fetch('/api/chat', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                message: message,
                session: session,
                files: files,
                stream: false
            })
        });
        
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
        }
        
        const data = await response.json();
        
        // Save session
        if (data.session) {
            localStorage.setItem('webui_session', data.session);
        }
        
        return data;
    } catch (error) {
        console.error('sendMessageWithFiles error:', error);
        throw error;
    }
}

// Add file download link to message
function addFileLinksToMessage(content) {
    // Match file paths in format: /workspace/.../filename.ext
    // Convert to download links
    const workspacePath = '/Users/waplay/.picoclaw/workspace/';
    
    return content.replace(new RegExp(workspacePath.replace(/\//g, '\\/') + '([\\w\\-\\.\\/]+)', 'g'), (match, relativePath) => {
        const fileName = relativePath.split('/').pop();
        const sessionMatch = relativePath.match(/webui\/([^\/]+)\//);
        const session = sessionMatch ? sessionMatch[1] : '';
        
        // Create download link
        const downloadUrl = `/api/files/download/${session}/${fileName}`;
        return `<a href="${downloadUrl}" class="file-link" download="${fileName}">📎 ${fileName}</a>`;
    });
}

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', initFileUpload);
