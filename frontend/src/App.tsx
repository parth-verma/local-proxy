import  { useState } from 'react'

function App() {
  const [name, setName] = useState<string>('');
  const [result] = useState<string>('Please enter your name below 👇');
  const [time] = useState<string>('Listening for Time event...');

  const doGreet = () => {
    let localName = name;
    if (!localName) {
      localName = 'anonymous';
    }
  }

  return (
      <div className="container">
        <div>
          <a data-wml-openURL="https://wails.io">
            <img src="/wails.png" className="logo" alt="Wails logo"/>
          </a>
          <a data-wml-openURL="https://reactjs.org">
            <img src="/react.svg" className="logo react" alt="React logo"/>
          </a>
        </div>
        <h1>Wails + React</h1>
        <div className="result">{result}</div>
        <div className="card">
          <div className="input-box">
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} type="text" autoComplete="off"/>
            <button className="btn" onClick={doGreet}>Greet</button>
          </div>
        </div>
        <div className="footer">
          <div><p>Click on the Wails logo to learn more</p></div>
          <div><p>{time}</p></div>
        </div>
      </div>
  )
}

export default App
