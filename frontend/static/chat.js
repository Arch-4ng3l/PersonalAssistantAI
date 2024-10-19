document.addEventListener("DOMContentLoaded", () => {
    const openChatButton = document.getElementById("open-chat-button");
    const chatModal = document.getElementById("chat-modal");
    const closeChatButton = document.getElementById("close-chat-button");
    const sendButton = document.getElementById("send-button");
    const messageInput = document.getElementById("message-input");
    const chatMessages = document.getElementById("chat-messages");

    // Function to open the chat modal
    openChatButton.addEventListener("click", () => {
        chatModal.style.display = "block";
        messageInput.focus(); // Focus on the input field when opened
    });

    // Function to close the chat modal
    closeChatButton.addEventListener("click", () => {
        chatModal.style.display = "none";
    });

    // Close the modal when clicking outside the chat container
    window.addEventListener("click", (event) => {
        if (event.target == chatModal) {
            chatModal.style.display = "none";
        }
    });

    // Function to append a message to the chat
    function appendMessage(sender, contentElement) {
        const messageElement = document.createElement("div");
        messageElement.classList.add("message", sender);

        // Create avatar
        const avatar = document.createElement("div");
        avatar.classList.add("avatar");
        const icon = document.createElement("i");
        const iconClass = sender !== "user" ? "bi-cpu" : "bi-person-circle";
        icon.classList.add("bi");
        icon.classList.add(iconClass);
        avatar.appendChild(icon);

        // Append avatar and message based on sender
        if (sender === "user") {
            messageElement.appendChild(createMessageContent(contentElement));
            messageElement.appendChild(avatar);
        } else {
            messageElement.appendChild(avatar);
            messageElement.appendChild(createMessageContent(contentElement));
        }

        chatMessages.appendChild(messageElement);

        // Scroll to the bottom
        chatMessages.scrollTop = chatMessages.scrollHeight;
    }

    // Helper function to create message content
    function createMessageContent(content) {
        if (typeof content === 'string') {
            const messageContent = document.createElement("div");
            messageContent.classList.add("message-content");
            messageContent.textContent = content;
            // Add timestamp
            const timestamp = document.createElement("div");
            timestamp.style.fontSize = "10px";
            timestamp.style.color = "#999";
            timestamp.style.textAlign = "right";
            const now = new Date();
            timestamp.textContent = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            messageContent.appendChild(timestamp);
            return messageContent;
        } else {
            // If content is an element (e.g., typing indicator)
            return content;
        }
    }

    // Function to show typing indicator
    function showTypingIndicator() {
        const typingIndicator = document.createElement("div");
        typingIndicator.classList.add("typing-indicator");

        for (let i = 0; i < 3; i++) {
            const dot = document.createElement("div");
            dot.classList.add("dot");
            typingIndicator.appendChild(dot);
        }

        appendMessage("ai", typingIndicator);
        return typingIndicator;
    }

    // Function to send message to backend
    async function sendMessage() {
        const message = messageInput.value.trim();
        if (message === "") return;

        // Append user message
        appendMessage("user", message);
        messageInput.value = "";

        // Show typing indicator
        const typingIndicator = showTypingIndicator();

        try {
            // Send the message to the backend
            const response = await fetch("/api/ai-chat", { // Replace with your backend URL
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ message }),
            });

            if (!response.ok) {
                throw new Error(`Error: ${response.statusText}`);
            }

            const data = await response.json();
            const aiMessage = data.reply; // Adjust based on your backend response structure

            
            chatMessages.removeChild(typingIndicator.parentNode);

            // Append AI message with text generation effect
            appendAIMessage(aiMessage);
        } catch (error) {
            console.error("Error sending message:", error);
            // Remove typing indicator
            chatMessages.removeChild(typingIndicator);
            appendMessage("ai", "Sorry, there was an error processing your request.");
        }
    }

    // Function to append AI message with typing effect
    function appendAIMessage(message) {
        const messageContent = document.createElement("div");
        messageContent.classList.add("message-content");

        const typingText = document.createElement("span");
        typingText.innerHTML = message
        messageContent.appendChild(typingText);

        appendMessage("ai", messageContent);


    }

    // Event listener for send button
    sendButton.addEventListener("click", sendMessage);

    // Event listener for Enter key
    messageInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
            sendMessage();
        }
    });
});
