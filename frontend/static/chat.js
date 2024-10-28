document.addEventListener("DOMContentLoaded", () => {
    const openChatButton = document.getElementById("open-chat-button");
    const chatModal = document.getElementById("chat-modal");
    const closeChatButton = document.getElementById("close-chat-button");
    const sendButton = document.getElementById("send-button");
    const messageInput = document.getElementById("message-input");
    const chatMessages = document.getElementById("chat-messages");

    // Add this function to handle textarea auto-resize
    function autoResizeTextArea(textarea) {
        // Reset height to auto to get the correct scrollHeight
        textarea.style.height = 'auto';
        
        // Calculate maximum height (4 lines approximately)
        const lineHeight = parseInt(window.getComputedStyle(textarea).lineHeight);
        const maxHeight = lineHeight * 4; // Adjust number of lines here
        
        // Set new height based on content, but not exceeding maxHeight
        const newHeight = Math.min(textarea.scrollHeight, maxHeight);
        textarea.style.height = newHeight + 'px';
    }

    // Function to open the chat modal
    openChatButton.addEventListener("click", () => {
        chatModal.style.display = "block";
        messageInput.focus(); // Focus on the input field when opened
        document.getElementById('calendar').classList.add('chat-open');

    });

    // Function to close the chat modal
    closeChatButton.addEventListener("click", () => {
        chatModal.style.display = "none";
        document.getElementById('calendar').classList.remove('chat-open');
    });

    // Close the modal when clicking outside the chat container
    window.addEventListener("click", (event) => {
        if (event.target == chatModal) {
            chatModal.style.display = "none";
            document.getElementById('calendar').classList.remove('chat-open');
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
            const aiMessage = data;
            console.log(aiMessage);
            console.log(response);

            
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
    async function appendAIMessage(message) {
        // Parse the message JSON if it's a JSON string
        try {
            const jsonMessage = JSON.parse(message);
            if (jsonMessage.message) {
                message = jsonMessage.message;
            }

            if (jsonMessage.action === "add_event") {
                const details = jsonMessage.details;
                if (confirm(`Would you like to add this event to your calendar?\n\nTitle: ${details.title}\nStart: ${new Date(details.startTime).toLocaleString()}\nEnd: ${new Date(details.endTime).toLocaleString()}`)) {
                    // Create calendar event
                    const eventInfo = {
                        startStr: details.startTime,
                        endStr: details.endTime
                    };
                    const id = getRandomHex32();
                    fetch("/api/calendar-create", {
                        method: "POST",
                        headers: {
                            "Content-Type": "application/json"
                        },
                        body: getGoogleEvent(eventInfo, details.title, id)
                    });
                    
                    calendar.addEvent({
                        title: details.title,
                        start: details.startTime,
                        end: details.endTime,
                        allDay: false,
                        id: id
                    });
                }
            }

        } catch (e) {
            // Not JSON, use message as-is
            console.log(e);
            console.log(message);
            console.log("Message is not JSON");
        }

        const messageContent = document.createElement("div");
        messageContent.classList.add("message-content");

        const typingText = document.createElement("span");
        messageContent.appendChild(typingText);


        appendMessage("ai", messageContent);
        const delay = 20;
        for (let i = 0; i < message.length; i++) {
            await new Promise(resolve => setTimeout(resolve, 100));
            typingText.textContent += message[i];
            chatMessages.scrollTop = chatMessages.scrollHeight;
        }
    }

    // Initialize textarea with single line height
    messageInput.style.height = 'auto';
    
    // Add input event listener for auto-resize
    messageInput.addEventListener('input', () => {
        autoResizeTextArea(messageInput);
    });

    // Event listener for send button
    sendButton.addEventListener("click", sendMessage);

    // Update Enter key handler
    messageInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) { // Allow Shift+Enter for new line
            e.preventDefault(); // Prevent default enter behavior
            sendMessage();
            // Reset cursor position and height
            messageInput.value = '';
            messageInput.style.height = 'auto';
        }
    });

    // Settings Modal Elements
    const openSettingsButton = document.getElementById("open-settings-button");
    const settingsModal = document.getElementById("settings-modal");
    const closeSettingsButton = document.getElementById("close-settings-button");
    const manageSubscriptionButton = document.getElementById("manage-subscription");
    const cancelSubscriptionButton = document.getElementById("cancel-subscription");

    // Function to open settings modal
    openSettingsButton.addEventListener("click", () => {
        settingsModal.style.display = "block";
        loadSubscriptionDetails();
    });

    // Function to close settings modal
    closeSettingsButton.addEventListener("click", () => {
        settingsModal.style.display = "none";
    });

    // Close modal when clicking outside
    window.addEventListener("click", (event) => {
        if (event.target == settingsModal) {
            settingsModal.style.display = "none";
        }
    });

    // Load subscription details
    async function loadSubscriptionDetails() {
        const subscriptionDetails = document.getElementById("subscription-details");
        try {
            const response = await fetch("/api/subscription-status");
            if (!response.ok) {
                throw new Error("Failed to load subscription details");
            }
            const data = await response.json();
            
            subscriptionDetails.innerHTML = `
                <p><strong>Plan:</strong> ${data.plan}</p>
                <p><strong>Status:</strong> ${data.status}</p>
                <p><strong>Next billing date:</strong> ${new Date(data.nextBillingDate).toLocaleDateString()}</p>
                <p><strong>Amount:</strong> $${data.amount}/month</p>
            `;
        } catch (error) {
            subscriptionDetails.innerHTML = `
                <p class="error">Error loading subscription details. Please try again later.</p>
            `;
        }
    }

    // Manage subscription
    manageSubscriptionButton.addEventListener("click", async () => {
        try {
            const response = await fetch("/api/subscription-portal");
            if (!response.ok) {
                throw new Error("Failed to get portal URL");
            }
            const data = await response.json();
            window.location.href = data.url;
        } catch (error) {
            alert("Failed to get portal URL");
        }
    });

    // Add this to your existing event listeners section
    cancelSubscriptionButton.addEventListener("click", async () => {
        if (confirm("Are you sure you want to cancel your subscription? This action cannot be undone.")) {
            try {
                const response = await fetch("/api/paypal-cancel", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({
                        reason: "User requested cancellation"
                    })
                });

                if (!response.ok) {
                    throw new Error("Failed to cancel subscription");
                }

                const data = await response.json();
                alert(data.message);
                // Reload subscription details to show updated status
                loadSubscriptionDetails();
            } catch (error) {
                alert("Error canceling subscription: " + error.message);
            }
        }
    });
});
