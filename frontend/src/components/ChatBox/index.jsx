import React, { useState, useEffect, useRef } from "react";
import Container from "@material-ui/core/Container";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import List from "@material-ui/core/List";
import TextField from "@material-ui/core/TextField";
import Typography from "@material-ui/core/Typography";
import Button from "@material-ui/core/Button";
import { makeStyles } from "@material-ui/core/styles";

const useStyles = makeStyles((theme) => ({
  root: {
    display: "flex",
    flexDirection: "column",
    maxWidth: 600,
    margin: 0,
    paddingBottom: theme.spacing(4),
  },
  messageList: {
    padding: theme.spacing(2),
    display: "flex",
    overflow: "auto",
    flexDirection: "column",
    height: 400,
    minHeight: 400,
  },
  bold: {
    fontWeight: "bold",
  },
  messageContainer: {
    maxWidth: "100%",
    paddingTop: theme.spacing(1),
    paddingBottom: theme.spacing(1),
    display: "inline-block",
  },
  otherMessage: {
    maxWidth: "70%",
    padding: theme.spacing(1),
    float: "left",
    backgroundColor: "#F6F6F6",
  },
  myMessage: {
    maxWidth: "70%",
    padding: theme.spacing(1),
    float: "right",
    backgroundColor: "#ADE9F1",
  },
  joinEvent: {
    maxWidth: "70%",
    padding: theme.spacing(1),
    margin: "auto",
    backgroundColor: "#78F5A5",
  },
  leaveEvent: {
    maxWidth: "70%",
    padding: theme.spacing(1),
    margin: "auto",
    backgroundColor: "#FF9B83",
  },
  messageSend: {
    height: "80%",
  },
  sendButton: {
    height: "100%",
    width: "100%",
  },
}));

const serverHost = process.env.REACT_APP_SERVER_HOST || "localhost:8080";

const useMessage = (socketRef) => {
  const [message, setMessage] = useState("");
  const handleSubmit = () => {
    if (message === "") return;
    socketRef.current.send(message);
    setMessage("");
  };
  return [message, setMessage, handleSubmit];
};

const useScrollOnNewEvent = (events) => {
  useEffect(() => {
    document
      .getElementById("scroll-to-bottom")
      .scrollIntoView({ behavior: "smooth" });
  }, [events]);
};

const useWebSocket = (username) => {
  let socketRef = useRef();
  const [events, setEvents] = useState([]);

  useEffect(() => {
    const socket = new WebSocket(
      `ws://${serverHost}/chat?username=${username}&sessionid=${Date.now()}`
    );
    socket.addEventListener("message", (event) => {
      setEvents((events) =>
        sortAndFilterEvents([...events, JSON.parse(event.data)])
      );
    });
    socket.addEventListener("close", () => {
      alert("you have been disconnected from the chat");
    });
    socketRef.current = socket;

    fetch(`http://${serverHost}/history`)
      .then((resp) => resp.json())
      .then((data) => {
        setEvents((events) => sortAndFilterEvents([...events, ...data.events]));
      })
      .catch((err) => console.log(err));

    return () => socket.close(1001);
  }, [socketRef, username, setEvents]);

  return [events, socketRef];
};

const ChatBox = ({ username }) => {
  const classes = useStyles();

  const [events, socketRef] = useWebSocket(username);
  const [message, setMessage, handleSubmit] = useMessage(socketRef);
  useScrollOnNewEvent(events);

  return (
    <Container component="main" className={classes.root}>
      <Typography component="h2" variant="h6" color="primary" gutterBottom>
       { `Go Chat - Logged in as: ${username}` }
      </Typography>
      <Grid container spacing={3}>
        <Grid item xs={12}>
          <Paper id="message-list" className={classes.messageList}>
            <List>
              {events.map((event) => (
                <Container key={event.id} className={classes.messageContainer}>
                  {outputMessage(username, event)}
                </Container>
              ))}
              <div id="scroll-to-bottom" />
            </List>
          </Paper>
        </Grid>
        <Grid item xs={12}>
          <Grid container spacing={3} className={classes.messageSend}>
            <Grid item xs={10} sm={9}>
              <TextField
                id="message-input"
                label="message"
                fullWidth
                variant="outlined"
                value={message}
                onKeyPress={(e) => {
                  if (e.key === "Enter") handleSubmit();
                }}
                onChange={(e) => setMessage(e.target.value)}
              />
            </Grid>
            <Grid item xs={2} sm={3}>
              <Button
                className={classes.sendButton}
                color="primary"
                variant="outlined"
                onClick={handleSubmit}
              >
                Send
              </Button>
            </Grid>
          </Grid>
        </Grid>
      </Grid>
    </Container>
  );
};

const sortAndFilterEvents = (events) => {
  // remove duplicates
  const eventsMap = {};
  events.forEach((event) => {
    eventsMap[event.id] = event;
  });
  events = Object.values(eventsMap);

  const sorted = events.sort((a, b) => {
    if (a.id < b.id) return -1;
    if (a.id > b.id) return 1;
    return 0;
  });
  return sorted;
};

const JoinMessage = ({ event }) => {
  const classes = useStyles();

  return (
    <Paper className={classes.joinEvent}>
      <li>{event.userId} has joined the chat</li>
    </Paper>
  );
};

const LeaveMessage = ({ event }) => {
  const classes = useStyles();

  return (
    <Paper className={classes.leaveEvent}>
      <li>{event.userId} has left the chat</li>
    </Paper>
  );
};

const CloseMessage = ({ event }) => {
  const classes = useStyles();

  return (
    <Paper className={classes.leaveEvent}>
      <li>the chat has ended</li>
    </Paper>
  );
};

const TextMessage = ({ event, username }) => {
  const classes = useStyles();

  return (
    <Paper
      className={
        event.userId === username ? classes.myMessage : classes.otherMessage
      }
    >
      <li>
        {event.userId === username ? (
          event.message
        ) : (
          <React.Fragment>
            <span className={classes.bold}>{event.userId}: </span>
            {event.message}
          </React.Fragment>
        )}
      </li>
    </Paper>
  );
};

const outputMessage = (username, event) => {
  let MessageComponent;
  switch (event.type) {
    case "message":
      MessageComponent = TextMessage;
      break;
    case "join":
      MessageComponent = JoinMessage;
      break;
    case "leave":
      MessageComponent = LeaveMessage;
      break;
    case "close":
      MessageComponent = CloseMessage;
      break;
    default:
      return null
  }

  return <MessageComponent username={username} event={event} />;
};

export default ChatBox;
