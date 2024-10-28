document.addEventListener("DOMContentLoaded", () => {
    const emailListContainer = document.getElementById("email-list");
    const statusDiv = document.getElementById("status");

    // Function to create and return an email element
    const createEmailElement = (email) => {
        const emailDiv = document.createElement("div");
        emailDiv.classList.add("email-list-element");

        // Subject
        const subject = document.createElement("h3");
        subject.classList.add("email-subject");
        subject.textContent = email.subject || "(No Subject)";
        emailDiv.appendChild(subject);

        // Body
        const body = document.createElement("div");
        body.classList.add("email-body");
        if(email.type == "text"){
            body.innerHTML = formatEmail(email.body) || "(No Content)";
        } else {
            body.innerHTML = email.body || "(No Content)";
        }
        emailDiv.appendChild(body);

        return emailDiv;
    };

    // Function to fetch emails
    const fetchEmails = async () => {
        try {
            const response = await fetch("/api/email", {
                method: "GET",
                headers: {
                    "Content-Type": "application/json"
                }
            });

            if (!response.ok) {
                throw new Error(`Server error: ${response.status} ${response.statusText}`);
            }

            const data = await response.json();

            // Validate data structure
            if (!data.items || !Array.isArray(data.items)) {
                throw new Error("Invalid data format received from server.");
            }

            // Filter out null or undefined items
            const validEmails = data.items.filter(item => item != null);

            // Clear any existing emails
            emailListContainer.innerHTML = "";

            // Populate the email list
            validEmails.forEach(email => {
                const emailElement = createEmailElement(email);
                emailListContainer.appendChild(emailElement);
            });

            // Update status
            statusDiv.textContent = `Loaded ${validEmails.length} emails.`;
            statusDiv.classList.remove("error");
        } catch (error) {
            console.error("Error fetching emails:", error);
            statusDiv.textContent = `Failed to load emails: ${error.message}`;
            statusDiv.classList.add("error");
        }
    };

    // Initial fetch
    fetchEmails();

    // Optional: Polling to refresh emails every 60 seconds
    // setInterval(fetchEmails, 60000);
});


function linkify(text) {
    const urlPattern = /(\b(https?|ftp|file):\/\/[-A-Z0-9+&@#\/%?=~_|!:,.;]*[-A-Z0-9+&@#\/%=~_|])/ig;
    return text.replace(urlPattern, '<a href="$1" target="_blank">$1</a>');
}

/**
 * Function to convert markdown-like syntax to HTML.
 * Handles:
 * - **bold text**
 * - * bullet points
 * - Replaces horizontal lines
 * @param {string} text - The input text with markdown-like syntax.
 * @returns {string} - The text converted to HTML.
 */
function formatEmail(text) {
    let html = linkify(text);

    // Replace **text** with <strong>text</strong>
    const boldPattern = /\*\*(.*?)\*\*/g;
    html = html.replace(boldPattern, '<strong>$1</strong>');

    // Replace * bullet points with <ul><li></li></ul>
    // First, split the text into lines
    const lines = html.split('\n');
    let formatted = '';
    let inList = false;

    lines.forEach(line => {
        line = line.trim();
        if (line.startsWith('* ')) {
            if (!inList) {
                inList = true;
                formatted += '<ul>';
            }
            const listItem = line.substring(2).trim();
            formatted += `<li>${listItem}</li>`;
        } else if (line.startsWith('- ') || line.startsWith('• ')) {
            if (!inList) {
                inList = true;
                formatted += '<ul>';
            }
            const listItem = line.substring(2).trim();
            formatted += `<li>${listItem}</li>`;
        } else {
            if (inList) {
                formatted += '</ul>';
                inList = false;
            }
            // Replace multiple dashes with horizontal rule
            if (/^[-]{3,}$/.test(line)) {
                formatted += '<hr>';
            } else {
                // Replace line breaks with <br>
                formatted += `<p>${line}</p>`;
            }
        }
    });

    if (inList) {
        formatted += '</ul>';
    }

    return formatted;
}