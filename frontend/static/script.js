function getRandomHex32() {
  const array = new Uint8Array(16); // 16 bytes = 128 bits
  window.crypto.getRandomValues(array);
  return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
}

function getGoogleEvent(info, title, id) {
    console.log(info)
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const startDate = new Date(info.startStr).toISOString();
    const endDate = new Date(info.endStr).toISOString();

    let f =  JSON.stringify({
        summary: title,
        start : {
            dateTime: startDate,
            timeZone: timeZone,
        },
        end: {
            dateTime: endDate,
            timeZone: timeZone,
        },
        id: id
    })
    return f
}

document.addEventListener('DOMContentLoaded', function() {
    var calendarEl = document.getElementById('calendar');

    var calendar = new FullCalendar.Calendar(calendarEl, {
        initialView: 'timeGridWeek',
        selectable: true,
        editable: true,
        events: [],
        headerToolbar: {
            left: 'prev,next today',
            center: 'title',
            right: 'timeGridWeek,timeGridDay'
        },
        slotDuration: '00:30:00',
        slotLabelInterval: '01:00',
        slotMinTime: '06:00:00',
        slotMaxTime: '24:00:00',
        themeSystem: 'bootstrap5',
        slotLabelFormat: {
            hour: "numeric",
            minute: "2-digit",
            hour12: false,
        },
        nowIndicator: true,
        select: function(info) {
            // Called when a date/time selection is made
            var title = prompt('Enter Event Title:');
            if (title) {
                id = getRandomHex32();
                calendar.addEvent({
                    title: title,
                    start: info.startStr,
                    end: info.endStr,
                    allDay: info.allDay,
                    id: id
                });

                fetch("/api/calendar-create", {
                    method: "POST",
                    body: getGoogleEvent(info, title, id),
                })
            }
            calendar.unselect();
        },
        eventClick: function(info) {
            if (confirm('Do you want to delete this event?')) {
                fetch("/api/calendar-remove", {
                    method: "POST",
                    body: getGoogleEvent(info.event, "", info.event.id),
                })
                info.event.remove();
            }
        }
    });

    fetch("/api/calendar-load", {
        method: "GET",
    })
        .then(response => {
            return response.json()
        })
        .then(data => {
            console.log(data);
            for(let i = 0; i < data["items"].length; i++) {
                calendar.addEvent({
                    title: data["items"][i].summary,
                    start: data["items"][i].start.dateTime,
                    end: data["items"][i].end.dateTime,
                    allDay: false,
                    id: data["items"][i].id

                });
            }
        })
    calendar.render();
});
