require("./index.html");
require("./main.css");

var IssueTag = React.createClass({
  render: function() {
    var level = ""
    if (this.props.level === "HIGH") {
      level = "is-danger";
    }
    if (this.props.level === "MEDIUM") {
      level = "is-warning";
    }
    return (
      <div className={ "tag " + level }>
        { this.props.label }: { this.props.level }
      </div>
    );
  }
});

var Issue = React.createClass({
	render: function() {
    return (
      <div className="issue box">
        <div className="is-pulled-right">
          <IssueTag label="Severity" level={ this.props.data.severity }/>
          <IssueTag label="Confidence" level={ this.props.data.confidence }/>
        </div>
        <p>
          <strong>
            { this.props.data.file } (line { this.props.data.line })
          </strong>
          <br/>
          { this.props.data.details.replace(/\.$/, "") }
        </p>
        <figure className="highlight">
          <pre>
            <code className="go hljs">
              { this.props.data.code }
            </code>
          </pre>
        </figure>
      </div>
    );
  }
});

var Issues = React.createClass({
  render: function() {
    if (this.props.data.results.issues.length === 0) {
      return (
        <div className="notification">
          Awesome! No issues found!
        </div>
      );
    }

		var issues = this.props.data.results.issues
      .filter(function(issue) {
        return this.props.severity.includes(issue.severity);
      }.bind(this))
      .filter(function(issue) {
        return this.props.confidence.includes(issue.confidence);
      }.bind(this))
      .map(function(issue) {
        return (<Issue data={issue} />);
		  });

    if (issues.length === 0) {
      return (
        <div className="notification">
          No issues matched given filters
          (of total { this.props.data.results.issues.length } issues).
        </div>
      );
    }

    return (
      <div className="issues">
        { issues }
        <p className="help">
          Last updated { new Date(this.props.data.time).toLocaleString() }.
          Scanned { this.props.data.results.metrics.files.toLocaleString() } files
          with { this.props.data.results.metrics.lines.toLocaleString() } lines of code.
        </p>
      </div>
    );
  }
});

var LevelSelector = React.createClass({
  handleChange: function(level) {
    return function(e) {
      var updated = this.props.selected
        .filter(function(item) { return item != level; });
      if (e.target.checked) {
        updated.push(level);
      }
      this.props.onChange(updated);
    }.bind(this);
  },
  render: function() {
    var highDisabled = !this.props.available.includes("HIGH");
    var mediumDisabled = !this.props.available.includes("MEDIUM");
    var lowDisabled = !this.props.available.includes("LOW");
 
    return (
      <span>
        <label className={"label checkbox " + (highDisabled ? "disabled" : "") }>
          <input
            type="checkbox"
            checked={ this.props.selected.includes("HIGH") }
            disabled={ highDisabled }
            onChange={ this.handleChange("HIGH") }/>
          High
        </label>
        <label className={"label checkbox " + (mediumDisabled ? "disabled" : "") }>
          <input
            type="checkbox"
            checked={ this.props.selected.includes("MEDIUM") }
            disabled={ mediumDisabled }
            onChange={ this.handleChange("MEDIUM") }/>
          Medium
        </label>
        <label className={"label checkbox " + (lowDisabled ? "disabled" : "") }>
          <input
            type="checkbox"
            checked={ this.props.selected.includes("LOW") }
            disabled={ lowDisabled }
            onChange={ this.handleChange("LOW") }/>
          Low
        </label>
      </span>
    );
  }
});

var Navigation = React.createClass({
  updateSeverity: function(values) {
    this.props.onSeverity(values);
  },
  updateConfidence: function(values) {
    this.props.onConfidence(values);
  },
  render: function() {
    return (
      <nav className="panel">
        <div className="panel-heading">
          Filters
        </div>
        <div className="panel-block">
          <span className="panel-icon">
            <i className="fa fa-exclamation-circle"></i>
				  </span>
          <strong>
            Severity
          </strong>
        </div>
        <div className="panel-block">
          <LevelSelector 
            selected={ this.props.severity }
            available={ this.props.allSeverities }
            onChange={ this.updateSeverity } />
        </div>
        <div className="panel-block">
          <span className="panel-icon">
            <i className="fa fa-question-circle"></i>
          </span>
          <strong>
            Confidence
          </strong>
        </div>
        <div className="panel-block">
          <LevelSelector
            selected={ this.props.confidence }
            available={ this.props.allConfidences }
            onChange={ this.updateConfidence } />
        </div>
      </nav>
    );
  }
});

var IssueBrowser = React.createClass({
  getInitialState: function() {
    return {};
  },
  componentDidMount: function() {
    this.loadIssues(this.props.repo);
  },
  handleSeverity: function(val) {
    this.setState({severity: val});
  },
  handleConfidence: function(val) {
    this.setState({confidence: val});
  },
  loadIssues: function() {
    $.ajax({
      url: "/results/github.com/" + this.props.repo,
      dataType: "json",
      cache: true,
      success: function(data) {
        this.updateIssues(data);
      }.bind(this),
      error: function(xhr, status, err) {
        this.setState({error: err.toString()});
      }.bind(this)
    });
  },
  updateIssues: function(data) {
    if (data.processing) {
      // Try again in 1s
      setTimeout(this.loadIssues, 1000);
    }

    if (!data.results) {
      this.setState({data: data});
      return;
    }

    var allSeverities = data.results.issues
      .map(function(issue) {
        return issue.severity
      })
      .sort()
      .filter(function(item, pos, ary) {
        return !pos || item != ary[pos - 1];
      });

    var allConfidences = data.results.issues
      .map(function(issue) {
        return issue.confidence
      })
      .sort()
      .filter(function(item, pos, ary) {
        return !pos || item != ary[pos - 1];
      });

    this.setState({
      data: data,
      severity: allSeverities.filter(function(i) { return i != "LOW" }),
      confidence: allConfidences.filter(function(i) { return i != "LOW" }),
      allSeverities: allSeverities,
      allConfidences: allConfidences
    });
  },
  render: function() {
    if (this.state.error) {
      return (
        <div className="content has-text-centered">
          <div className="notification is-danger">
            Uh oh! We encountered an error: { this.state.error }.
          </div>
        </div>
      );
    }

    if (this.state.data === undefined || this.state.data.processing) {
      // Still loading and/or processing on backend
      return (
        <div className="content has-text-centered">
          <div className="button is-large borderless is-loading">
          </div>
        </div>
      );
    }

    return (
      <div className="content">
        <h2 className="subtitle">
          results for { this.state.data.repo }
          <a href={ "https://" + this.state.data.repo } className="icon is-pulled-right borderless">
            <i className="fa fa-github-alt" aria-hidden="true"></i>
          </a>
        </h2>
        <hr/>
        <div className="columns">
          <div className="column is-one-quarter">
            <Navigation
              severity={ this.state.severity } 
              confidence={ this.state.confidence }
              allSeverities={ this.state.allSeverities } 
              allConfidences={ this.state.allConfidences }
              onSeverity={ this.handleSeverity } 
              onConfidence={ this.handleConfidence } 
            />
          </div>
          <div className="column is-three-quarters">
            <Issues
              data={ this.state.data }
              severity={ this.state.severity }
              confidence={ this.state.confidence }
            />
          </div>
        </div>
      </div>
    );
  }
});

var RepoSelector = React.createClass({
  getInitialState: function() {
    return { repo: "", valid: true };
  },
  updateRepo: function(e) {
    var re = /^[a-zA-Z-0-9-_.]+\/[a-zA-Z-0-9-_.]+$/;
    this.setState({
      repo: e.target.value,
      valid: this.state.valid || re.test(e.target.value)
    });
  },
  submitForm: function(e) {
    e.preventDefault();
    var re = /^[a-zA-Z-0-9-_.]+\/[a-zA-Z-0-9-_.]+$/;
    var valid = re.test(this.state.repo);
    if (!valid) {
      this.setState({valid: valid});
      return;
    }
    $.ajax({
      type: "POST",
      url: "/queue/github.com/" + this.state.repo,
      success: function(data) {
        location.href = '/#' + this.state.repo;
      }.bind(this),
      error: function(xhr, status, err) {
        this.setState({error: err.toString()});
      }.bind(this)
    });
  },
  render: function() {
    if (this.state.error) {
      return (
        <div className="content has-text-centered">
          <div className="notification is-danger">
            Uh oh! We encountered an error: { this.state.error }.
          </div>
        </div>
      );
    }

    if (!this.state.valid) {
      var warning = (
        <span className="help is-medium is-danger">
          Invalid input, should be 'user/repository'.
        </span>
      );
    }

    return (
      <div className="content">
        <h2 className="subtitle">
          run static analysis
        </h2>
        <hr/>
        <p>
          Hi! This is a simple online interface to
          run <a href="https://github.com/HewlettPackard/gas">gas</a> (a
          static analysis tool for Go) on your GitHub repository.
        </p>
        <p>
          Enter a user/repository tuple below and hit Enter to get started.
        </p>
        <form onSubmit={ this.submitForm }>
          <label className="label is-hidden">
            GitHub repository
          </label>
          <p className="control">
            <input
              className={ "input is-medium " + (this.state.valid ? "" : "is-danger")}
              type="text"
              value={ this.state.repo }
              onChange={ this.updateRepo }
              placeholder="enter user/repository"
            />
            { warning }
          </p>
        </form>
      </div>
    );
  }
});

function render() {
  var repo = window.location.hash.replace(/^#\/?|\/$/g, '');
  if (!repo) {
    ReactDOM.render(
      <RepoSelector />,
      document.getElementById("content")
    );
  } else {
    ReactDOM.render(
      <IssueBrowser repo={ repo } />,
      document.getElementById("content")
    );
  }
}

render();
window.addEventListener('hashchange', render, false);
