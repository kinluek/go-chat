import { CssBaseline } from "@material-ui/core";
import React, { useState } from "react";
import ChatBox from "./components/ChatBox";
import Container from "@material-ui/core/Container";
import { makeStyles } from "@material-ui/core/styles";
import Login from "./components/Login";

const useStyles = makeStyles((theme) => ({
  main: {
    display: "flex",
    maxWidth: 800,
    maxHeight: 1000,
    paddingTop: theme.spacing(4),
    paddingBottom: theme.spacing(4),
  },
}));

function App() {
  const [username, setUser] = useState("");
  const classes = useStyles();

  return (
    <React.Fragment>
      <CssBaseline />
      {username ? (
        <Container component="main" className={classes.main}>
          <ChatBox username={username} />
        </Container>
      ) : (
        <Login username={username} setUser={setUser}></Login>
      )}
    </React.Fragment>
  );
}

export default App;
