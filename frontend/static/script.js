function getRandomHex32() {
    const array = new Uint8Array(16);
    window.crypto.getRandomValues(array);
    return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
}

function getGoogleEvent(info, title, id) {
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const startTime = new Date(info.startStr).toISOString();
    const endTime = new Date(info.endStr).toISOString();

    return JSON.stringify({
        title: title,
        startTime: startTime,
        endTime: endTime,
        id: id
    });
}

function findEvent(title, start, end) {
    return calendar.getEvents().find(
        event => event.title === title && event.startStr === start && event.endStr == end 
    );
}

function showConfirmationModal(title, message, details, onConfirm) {
    const modal = document.getElementById('confirmation-modal');
    const detailsContainer = document.getElementById('confirmation-details');
    const confirmButton = document.getElementById('confirm-action');
    const cancelButton = document.getElementById('cancel-action');
    const closeButton = document.getElementById('close-confirmation-button');
    const modalTitle = modal.querySelector('h2');

    // Update modal content
    modalTitle.textContent = title;
    detailsContainer.innerHTML = `
        <div class="bg-background-dark rounded-lg p-4 space-y-3">
            <div class="text-gray-300">
                <span class="font-medium text-gray-200">${message}</span>
            </div>
            ${details}
        </div>
    `;

    // Show modal with animation
    modal.classList.remove('hidden');
    modal.classList.add('animate-fade-in');

    // Handle confirmation
    const handleConfirm = () => {
        modal.classList.add('animate-fade-out');
        setTimeout(() => {
            modal.classList.add('hidden');
            onConfirm();
            cleanup();
        }, 200);
    };

    // Handle cancel
    const handleCancel = () => {
        modal.classList.add('animate-fade-out');
        setTimeout(() => {
            modal.classList.add('hidden');
            cleanup();
        }, 200);
    };

    // Handle click outside
    const handleOutsideClick = (event) => {
        if (event.target === modal) {
            handleCancel();
        }
    };

    // Cleanup function
    const cleanup = () => {
        confirmButton.removeEventListener('click', handleConfirm);
        cancelButton.removeEventListener('click', handleCancel);
        closeButton.removeEventListener('click', handleCancel);
        window.removeEventListener('click', handleOutsideClick);
    };

    // Add event listeners
    confirmButton.addEventListener('click', handleConfirm);
    cancelButton.addEventListener('click', handleCancel);
    closeButton.addEventListener('click', handleCancel);
    window.addEventListener('click', handleOutsideClick);
}

document.addEventListener('DOMContentLoaded', function() {
    var calendarEl = document.getElementById('calendar');
    const eventModal = document.getElementById('event-modal');
    const eventTitleInput = document.getElementById('event-title');
    const closeModalButton = document.getElementById('close-event-modal');
    let selectedEventInfo = null;

    // Function to close the event modal
    function closeEventModal() {
        eventModal.classList.add('hidden');
        eventTitleInput.value = '';
        selectedEventInfo = null;
        calendar.unselect();
    }

    // Event listener for the close button
    closeModalButton.addEventListener('click', (e) => {
        e.preventDefault();
        closeEventModal();
    });

    // Close modal when clicking outside
    eventModal.addEventListener('click', (e) => {
        if (e.target === eventModal) {
            closeEventModal();
        }
    });

    // Add escape key listener
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && !eventModal.classList.contains('hidden')) {
            closeEventModal();
        }
    });

    calendar = new FullCalendar.Calendar(calendarEl, {
        locale: "de",
        firstDay: 1,
        initialView: 'timeGridWeek',
        selectable: true,
        editable: true,
        events: [],
        headerToolbar: {
            left: 'prev,next today',
            center: 'title',
            right: 'timeGridWeek,timeGridDay,listMonth'
        },
        // Calendar styling options
        height: 'auto',
        allDaySlot: false,
        expandRows: true,
        slotDuration: '00:30:00',
        slotLabelInterval: '01:00',
        slotMinTime: '06:00:00',
        slotMaxTime: '24:00:00',
        nowIndicator: true,

        // Modern theme customization
        themeSystem: 'standard',
        eventColor: '#2563eb', // Primary blue
        eventTextColor: '#ffffff',
        eventBorderColor: 'transparent',
        eventClassNames: 'rounded-lg shadow-md hover:shadow-lg transition-shadow',
        
        // Calendar UI colors
        views: {
            timeGrid: {
                // Slot styling (time slots)
                slotLabelFormat: {
                    hour: '2-digit',
                    minute: '2-digit',
                    hour12: false
                }
            }
        },
        
        // Custom CSS variables
        customButtons: {
            today: {
                text: 'Today',
                className: 'fc-button-primary hover:bg-primary-dark'
            }
        },

        // Calendar styling
        buttonText: {
            today: 'Today',
            week: 'Week',
            day: 'Day',
            list: 'List'
        },

        // Additional styling through CSS vars
        bootstrapFontAwesome: false,
        
        // Calendar content styling
        dayCellClassNames: 'hover:bg-gray-700/30 transition-colors',
        slotLabelClassNames: 'text-gray-400 font-medium',
        dayHeaderClassNames: 'text-gray-300 font-semibold uppercase text-sm',
        nowIndicatorClassNames: 'bg-red-500',
        
        // Event styling
        eventDidMount: function(info) {
            info.el.classList.add('transition-transform', 'hover:-translate-y-0.5', 'duration-200');
        },

        select: function(info) {
            selectedEventInfo = info;
            eventModal.classList.remove('hidden');
            eventTitleInput.focus();
        },
        
        eventClick: function(info) {
            const details = `
                <div class="space-y-2">
                    <div class="flex items-center justify-between text-gray-300">
                        <span class="font-medium">Event:</span>
                        <span>${info.event.title}</span>
                    </div>
                    <div class="flex items-center justify-between text-gray-300">
                        <span class="font-medium">Time:</span>
                        <span>${new Date(info.event.start).toLocaleString()} - ${new Date(info.event.end).toLocaleString()}</span>
                    </div>
                </div>
            `;

            showConfirmationModal(
                'Delete Event',
                'Are you sure you want to delete this event?',
                details,
                () => {
                    fetch("/api/calendar-remove", {
                        method: "POST",
                        body: getGoogleEvent(info.event, "", info.event.id),
                    });
                    info.event.remove();
                    showToast('Event has been deleted.');
                }
            );
        }
    });

    // Event creation handler
    document.getElementById('save-event').addEventListener('click', function(e) {
        e.preventDefault();
        const title = eventTitleInput.value.trim();
        if (title && selectedEventInfo) {
            const details = `
                <div class="space-y-2">
                    <div class="flex items-center justify-between text-gray-300">
                        <span class="font-medium">Title:</span>
                        <span>${title}</span>
                    </div>
                    <div class="flex items-center justify-between text-gray-300">
                        <span class="font-medium">Time:</span>
                        <span>${new Date(selectedEventInfo.startStr).toLocaleString()} - ${new Date(selectedEventInfo.endStr).toLocaleString()}</span>
                    </div>
                </div>
            `;

            showConfirmationModal(
                'Create Event',
                'Do you want to create this event?',
                details,
                () => {
                    const id = getRandomHex32();
                    calendar.addEvent({
                        title: title,
                        start: selectedEventInfo.startStr,
                        end: selectedEventInfo.endStr,
                        allDay: selectedEventInfo.allDay,
                        id: id
                    });

                    fetch("/api/calendar-create", {
                        method: "POST",
                        body: getGoogleEvent(selectedEventInfo, title, id),
                    });

                    closeEventModal();
                    showToast('Event has been created.');
                }
            );
        } else {
            showToast('Please enter a title.', 'error');
        }
    });

    // Custom toast function
    function showToast(message, type = 'success') {
        const toast = document.createElement('div');
        toast.className = `fixed bottom-4 left-1/2 transform -translate-x-1/2 
            px-6 py-3 rounded-lg shadow-lg text-white text-sm font-medium
            transition-all duration-300 opacity-0 translate-y-2
            ${type === 'success' ? 'bg-green-500' : 'bg-red-500'}`;
        toast.textContent = message;
        document.body.appendChild(toast);

        // Animate in
        setTimeout(() => {
            toast.classList.remove('opacity-0', 'translate-y-2');
        }, 10);

        // Animate out
        setTimeout(() => {
            toast.classList.add('opacity-0', 'translate-y-2');
            setTimeout(() => {
                document.body.removeChild(toast);
            }, 300);
        }, 2000);
    }

    // Load initial events
    fetch("/api/calendar-load", {
        method: "GET",
    })
        .then(response => response.json())
        .then(data => {
            data.items?.forEach(item => {
                calendar.addEvent({
                    title: item.title,
                    start: item.startTime,
                    end: item.endTime,
                    allDay: false,
                    id: item.id
                });
            });
        })
        .catch(error => {
            console.error('Error loading events:', error);
            showToast('Failed to load events.', 'error');
        });

    calendar.render();
});
