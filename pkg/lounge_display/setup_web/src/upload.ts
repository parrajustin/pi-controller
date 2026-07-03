import { LitElement, html, css } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import { WrapPromise } from 'standard-ts-lib/src/wrap_promise.js';

@customElement('upload-page')
export class UploadPage extends LitElement {
  static styles = css`
    :host {
      display: flex;
      width: 100vw;
      height: 100vh;
      margin: 0;
      background: linear-gradient(135deg, #0f172a 0%, #1e1b4b 100%);
      color: #e2e8f0;
      font-family: 'Google Sans', 'Inter', sans-serif;
      align-items: center;
      justify-content: center;
      overflow: hidden;
    }

    .container {
      width: 90%;
      max-width: 500px;
      padding: 40px;
      background: rgba(255, 255, 255, 0.05);
      backdrop-filter: blur(20px);
      border-radius: 24px;
      border: 1px solid rgba(255, 255, 255, 0.1);
      box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
      display: flex;
      flex-direction: column;
      align-items: center;
      text-align: center;
      transition: all 0.3s ease;
    }

    h1 {
      margin-top: 0;
      margin-bottom: 8px;
      font-size: 1.8rem;
      font-weight: 700;
      background: linear-gradient(to right, #818cf8, #c084fc);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    p {
      color: #94a3b8;
      margin-bottom: 32px;
      font-size: 1rem;
      line-height: 1.5;
    }

    .drop-zone {
      width: 100%;
      height: 200px;
      border: 2px dashed rgba(129, 140, 248, 0.4);
      border-radius: 16px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
      background: rgba(129, 140, 248, 0.02);
      position: relative;
      overflow: hidden;
    }

    .drop-zone:hover, .drop-zone.drag-active {
      border-color: #818cf8;
      background: rgba(129, 140, 248, 0.1);
      transform: scale(1.02);
    }
    
    .drop-zone.drag-active {
      box-shadow: 0 0 0 4px rgba(129, 140, 248, 0.2);
    }

    .upload-icon {
      width: 64px;
      height: 64px;
      margin-bottom: 16px;
      fill: #818cf8;
      transition: transform 0.3s ease;
    }

    .drop-zone:hover .upload-icon, .drop-zone.drag-active .upload-icon {
      transform: translateY(-8px);
    }

    .drop-text {
      font-size: 1.1rem;
      font-weight: 500;
      color: #cbd5e1;
    }

    .drop-subtext {
      font-size: 0.85rem;
      color: #64748b;
      margin-top: 8px;
    }

    input[type="file"] {
      display: none;
    }

    .status-message {
      margin-top: 24px;
      padding: 16px;
      border-radius: 12px;
      width: 100%;
      font-size: 0.95rem;
      font-weight: 500;
      box-sizing: border-box;
      opacity: 0;
      transform: translateY(10px);
      transition: all 0.4s ease;
    }
    
    .status-message.show {
      opacity: 1;
      transform: translateY(0);
    }

    .status-message.error {
      background: rgba(239, 68, 68, 0.1);
      color: #fca5a5;
      border: 1px solid rgba(239, 68, 68, 0.2);
    }

    .status-message.success {
      background: rgba(34, 197, 94, 0.1);
      color: #86efac;
      border: 1px solid rgba(34, 197, 94, 0.2);
    }

    .loader {
      border: 3px solid rgba(129, 140, 248, 0.2);
      border-top-color: #818cf8;
      border-radius: 50%;
      width: 40px;
      height: 40px;
      animation: spin 1s linear infinite;
      margin: 0 auto;
    }

    @keyframes spin {
      0% { transform: rotate(0deg); }
      100% { transform: rotate(360deg); }
    }
  `;

  @state() private dragActive = false;
  @state() private uploadState: 'idle' | 'uploading' | 'success' | 'error' = 'idle';
  @state() private errorMessage = '';
  @query('#fileInput') private fileInput!: HTMLInputElement;

  private handleDragEnter(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (this.uploadState !== 'uploading' && this.uploadState !== 'success') {
      this.dragActive = true;
    }
  }

  private handleDragLeave(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    this.dragActive = false;
  }

  private handleDragOver(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
  }

  private handleDrop(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    this.dragActive = false;
    
    if (this.uploadState === 'uploading' || this.uploadState === 'success') return;

    if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
      this.processFile(e.dataTransfer.files[0]);
    }
  }

  private handleFileSelect(e: Event) {
    const target = e.target as HTMLInputElement;
    if (target.files && target.files.length > 0) {
      this.processFile(target.files[0]);
    }
  }

  private openFileDialog() {
    if (this.uploadState !== 'uploading' && this.uploadState !== 'success') {
      this.fileInput.click();
    }
  }

  private async processFile(file: File) {
    if (!file.name.endsWith('.json')) {
      this.showError('Please upload a valid .json file.');
      return;
    }

    this.uploadState = 'uploading';
    this.errorMessage = '';

    try {
      const text = await file.text();
      // Verify it is JSON
      let data;
      try {
        data = JSON.parse(text);
      } catch (e) {
        this.showError('Invalid JSON format.');
        return;
      }

      // Check if it looks like a credentials file (basic check)
      if (!data.installed && !data.web) {
        this.showError('File does not appear to be a valid credentials.json from Google Cloud.');
        return;
      }

      // Upload to server
      const res = await WrapPromise(fetch('/api/cred', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: text
      }), 'Network error');

      if (!res.ok || !res.safeUnwrap().ok) {
        this.showError('Failed to upload credentials to the server.');
        return;
      }

      this.uploadState = 'success';
      
    } catch (err) {
      this.showError('An unexpected error occurred reading the file.');
    }
  }

  private showError(msg: string) {
    this.errorMessage = msg;
    this.uploadState = 'error';
    // Reset file input
    if (this.fileInput) this.fileInput.value = '';
  }

  render() {
    return html`
      <div class="container">
        <h1>Upload Credentials</h1>
        <p>Please upload your Google Cloud credentials.json file to complete the setup process.</p>
        
        ${this.uploadState === 'uploading' ? html`
          <div class="drop-zone" style="cursor: default;">
            <div class="loader"></div>
            <div class="drop-text" style="margin-top: 16px;">Uploading...</div>
          </div>
        ` : this.uploadState === 'success' ? html`
          <div class="drop-zone" style="cursor: default; border-color: rgba(34, 197, 94, 0.4); background: rgba(34, 197, 94, 0.05);">
            <svg class="upload-icon" style="fill: #4ade80;" viewBox="0 0 24 24">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
            </svg>
            <div class="drop-text" style="color: #86efac;">Upload Complete!</div>
            <div class="drop-subtext" style="color: #94a3b8;">You may now return to the display.</div>
          </div>
        ` : html`
          <div class="drop-zone ${this.dragActive ? 'drag-active' : ''}"
               @dragenter=${this.handleDragEnter}
               @dragleave=${this.handleDragLeave}
               @dragover=${this.handleDragOver}
               @drop=${this.handleDrop}
               @click=${this.openFileDialog}>
            <svg class="upload-icon" viewBox="0 0 24 24">
              <path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM14 13v4h-4v-4H7l5-5 5 5h-3z"/>
            </svg>
            <div class="drop-text">Click or drag credentials.json here</div>
            <div class="drop-subtext">Supported format: JSON</div>
          </div>
          <input type="file" id="fileInput" accept=".json" @change=${this.handleFileSelect} />
        `}

        <div class="status-message ${this.uploadState === 'error' ? 'error show' : this.uploadState === 'success' ? 'success show' : ''}">
          ${this.uploadState === 'error' ? this.errorMessage : this.uploadState === 'success' ? 'Credentials securely transferred to the device.' : ''}
        </div>
      </div>
    `;
  }
}
