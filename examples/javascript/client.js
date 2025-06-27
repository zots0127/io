const fs = require('fs');
const FormData = require('form-data');
const axios = require('axios');

class IOClient {
    constructor(baseURL = 'http://localhost:8080', apiKey = 'your-secure-api-key') {
        this.baseURL = baseURL;
        this.apiKey = apiKey;
        this.headers = { 'X-API-Key': apiKey };
    }

    async storeFile(filePath) {
        const form = new FormData();
        form.append('file', fs.createReadStream(filePath));
        
        const response = await axios.post(`${this.baseURL}/api/store`, form, {
            headers: {
                ...this.headers,
                ...form.getHeaders()
            }
        });
        
        return response.data.sha1;
    }

    async getFile(sha1, outputPath) {
        const response = await axios.get(`${this.baseURL}/api/file/${sha1}`, {
            headers: this.headers,
            responseType: 'stream'
        });
        
        const writer = fs.createWriteStream(outputPath);
        response.data.pipe(writer);
        
        return new Promise((resolve, reject) => {
            writer.on('finish', resolve);
            writer.on('error', reject);
        });
    }

    async deleteFile(sha1) {
        const response = await axios.delete(`${this.baseURL}/api/file/${sha1}`, {
            headers: this.headers
        });
        
        return response.data;
    }

    async fileExists(sha1) {
        const response = await axios.get(`${this.baseURL}/api/exists/${sha1}`, {
            headers: this.headers
        });
        
        return response.data.exists;
    }
}

// Example usage
async function example() {
    const client = new IOClient();
    
    // Create a test file
    fs.writeFileSync('test.txt', 'Hello from JavaScript!');
    
    try {
        // Store the file
        const sha1 = await client.storeFile('test.txt');
        console.log(`File stored with SHA1: ${sha1}`);
        
        // Check if file exists
        const exists = await client.fileExists(sha1);
        console.log(`File exists: ${exists}`);
        
        // Download the file
        await client.getFile(sha1, 'downloaded.txt');
        console.log('File downloaded successfully');
        
        // Read and verify content
        const content = fs.readFileSync('downloaded.txt', 'utf8');
        console.log(`Downloaded content: ${content}`);
        
        // Delete the file
        const result = await client.deleteFile(sha1);
        console.log('File deleted:', result);
        
    } finally {
        // Clean up
        fs.unlinkSync('test.txt');
        fs.unlinkSync('downloaded.txt');
    }
}

// Run example if called directly
if (require.main === module) {
    example().catch(console.error);
}

module.exports = IOClient;