function getRandomHex32() {
  const array = new Uint8Array(16); // 16 bytes = 128 bits
  window.crypto.getRandomValues(array);
  return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
}

function getGoogleEvent(info, title, id) {
    console.log(info)
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    const startTime = new Date(info.startStr).toISOString();
    const endTime = new Date(info.endStr).toISOString();

    let f =  JSON.stringify({
        title: title,
        startTime: startTime,
        endTime: endTime,
        id: id
    })
    return f
}
var calendar;


function findEvent(title, start, end) {
    let event = calendar.getEvents().find(
        event => event.title === title && event.startStr === start && event.endStr == end 
    )
    return event;
}

function moveEventButton(title, start, end, movedStart, movedEnd) {
    let event = findEvent(title, start, end);

    if (confirm('Do you want to move this event?')) {
        fetch("/api/calendar-remove", {
            method: "POST",
            body: getGoogleEvent(event, "", event.id),
        })
        event.remove();
        M.toast({html: 'Das Ereignis wurde gelöscht.', displayLength: 2000});
    }

    let id  = getRandomHex32();
    calendar.addEvent({
        title: title,
        start: movedStart,
        end: movedEnd,
        allDay: false,
        id: id,
    });

    let info = {
        startStr: movedStart,
        endStr: movedEnd,
    }

    fetch("/api/calendar-create", {
        method: "POST",
        body: getGoogleEvent(info, title, id),
    })
    document.getElementById(title).disabled = true;

}

function removeEventButton(title, start, end) {
    let event = findEvent(title, start, end);

    if (confirm('Do you want to delete this event?')) {
        fetch("/api/calendar-remove", {
            method: "POST",
            body: getGoogleEvent(event, "", event.id),
        })
        event.remove();
        M.toast({html: 'Das Ereignis wurde gelöscht.', displayLength: 2000});
        document.getElementById(title).disabled = true;
    }
}

function addEventButton(title, start, end) {
    let id  = getRandomHex32();
    calendar.addEvent({
        title: title,
        start: start,
        end: end,
        allDay: false,
        id: id,
    });

    let info = {
        startStr: start,
        endStr: end,
    }
    console.log(title, start, end);

    fetch("/api/calendar-create", {
        method: "POST",
        body: getGoogleEvent(info, title, id),
    })
    document.getElementById(title).disabled = true;
}

var selectedEventInfo;
var eventModal;

document.addEventListener('DOMContentLoaded', function() {


    console.log(M);
    M.AutoInit();
    var modals = document.querySelectorAll(".modal");
    console.log(M.Modal);
    M.Modal.init(modals);

    eventModal = M.Modal.getInstance(document.getElementById('event-modal'));

    var calendarEl = document.getElementById('calendar');

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
        allDaySlot: false,
        expandRows: false,
        updateSize: true,
        handleWindowResize: true,
        slotDuration: '00:30:00',
        slotLabelInterval: '01:00',
        slotMinTime: '06:00:00',
        slotMaxTime: '24:00:00',
        themeSystem: 'standard',
        nowIndicator: true,
        contentHeight: 'auto',
        select: function(info) {
            // Called when a date/time selection is made
            selectedEventInfo = info;
            eventModal.open();
        },
        eventClick: function(info) {
            if (confirm('Do you want to delete this event?')) {
                fetch("/api/calendar-remove", {
                    method: "POST",
                    body: getGoogleEvent(info.event, "", info.event.id),
                })
                info.event.remove();
                M.toast({html: 'Das Ereignis wurde gelöscht.', displayLength: 2000});
            }
        }
    });


    document.getElementById('save-event').addEventListener('click', function() {
        var title = document.getElementById('event-title').value;
        if (title) {
            var id = getRandomHex32();
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

            // Reset form and close modal
            document.getElementById('event-title').value = '';
            eventModal.close();
        } else {
            M.toast({html: 'Bitte geben Sie einen Titel ein.', displayLength: 2000});
        }
        calendar.unselect();
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
                console.log(data["items"]);
                
                calendar.addEvent({
                    title: data["items"][i].title,
                    start: data["items"][i].startTime,
                    end: data["items"][i].endTime,
                    allDay: false,
                    id: data["items"][i].id

                });
            }
        })
    calendar.render();
});
