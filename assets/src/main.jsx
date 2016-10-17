require("./index.html");
require("./main.css");

function cleanupIssueType(details) {
    return (details.charAt(0).toUpperCase() + details.slice(1)).replace(/\.$/, "");
}

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
  componentDidMount: function() {
    hljs.highlightBlock(ReactDOM.findDOMNode(this).querySelector("pre code"));
  },
  componentDidUpdate: function() {
    hljs.highlightBlock(ReactDOM.findDOMNode(this).querySelector("pre code"));
  },
  render: function() {
    return (
      <div className="issue box">
        <div className="is-pulled-right">
          <IssueTag label="Severity" level={ this.props.data.severity }/>
          <IssueTag label="Confidence" level={ this.props.data.confidence }/>
        </div>
        <p>
          <strong>
            <a className="icon borderless is-pulled-right"
               href={ this.props.path + "/" + this.props.data.file + "#L" + this.props.data.line }
               alt="Jump to code">
               <i className="fa fa-code" aria-hidden="true"></i>
            </a>
            <a className="issue-title borderless"
               href={ this.props.path + "/" + this.props.data.file + "#L" + this.props.data.line }>
               { this.props.data.file } (line { this.props.data.line })
            </a>
          </strong>
          <br/>
          { cleanupIssueType(this.props.data.details) }
        </p>
        <figure className="highlight">
          <pre>
            <code className="golang hljs">
              { this.props.data.code }
            </code>
          </pre>
        </figure>
      </div>
    );
  }
});

var Stats = React.createClass({
  render: function() {
    return (
      <p className="help">
        Last updated { new Date(this.props.data.time).toLocaleString() }.
        Scanned { this.props.data.results.metrics.files.toLocaleString() } files
        with { this.props.data.results.metrics.lines.toLocaleString() } lines of code.
      </p>
    );
  }
});

var Issues = React.createClass({
  render: function() {
    if (this.props.data.results.metrics.files === 0) {
      return (
        <div className="notification">
          No source files found. Do you even Go?
        </div>
      );
    }

    if (this.props.data.results.issues.length === 0) {
      return (
        <div>
          <div className="notification">
            Awesome! No issues found!
          </div>
          <Stats data={ this.props.data } />
        </div>
      );
    }

    var tag = this.props.data.tag;
    if (!tag) {
      tag = "master";
    }

    var repoPath =
      "https://" + this.props.data.repo  + "/blob/" + tag;

    var issues = this.props.data.results.issues
      .filter(function(issue) {
        return this.props.severity.includes(issue.severity);
      }.bind(this))
      .filter(function(issue) {
        return this.props.confidence.includes(issue.confidence);
      }.bind(this))
      .filter(function(issue) {
        if (this.props.issueType) {
          return issue.details.toLowerCase().startsWith(this.props.issueType.toLowerCase());
        } else {
          return true
        }
      }.bind(this))
      .map(function(issue) {
        return (<Issue path={repoPath} data={issue} />);
      }.bind(this));

    if (issues.length === 0) {
      return (
        <div>
          <div className="notification">
            No issues matched given filters
            (of total { this.props.data.results.issues.length } issues).
          </div>
          <Stats data={ this.props.data } />
        </div>
      );
    }

    return (
      <div className="issues">
        { issues }
        <Stats data={ this.props.data } />
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
  updateSeverity: function(vals) {
    this.props.onSeverity(vals);
  },
  updateConfidence: function(vals) {
    this.props.onConfidence(vals);
  },
  updateIssueType: function(e) {
    if (e.target.value == "all") {
      this.props.onIssueType(null);
    } else {
      this.props.onIssueType(e.target.value);
    }
  },
  render: function() {
    var issueTypes = this.props.allIssueTypes
      .map(function(it) {
        return (
          <option value={ it } selected={ this.props.issueType == it }>
            { it }
          </option>
        );
      }.bind(this));

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
        <div className="panel-block">
          <span className="panel-icon">
            <i className="fa fa-info-circle"></i>
          </span>
          <strong>
            Issue Type
          </strong>
        </div>
        <div className="panel-block">
          <select onChange={ this.updateIssueType }>
            <option value="all" selected={ !this.props.issueType }>
              (all)
            </option>
            { issueTypes }
          </select>
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
    this.updateIssueTypes(this.state.data.results.issues, val, this.state.confidence);
    this.setState({severity: val});
  },
  handleConfidence: function(val) {
    this.updateIssueTypes(this.state.data.results.issues, this.state.severity, val);
    this.setState({confidence: val});
  },
  handleIssueType: function(val) {
    this.setState({issueType: val});
  },
  loadIssues: function() {
    reqwest({
      url: "/results/github.com/" + this.props.repo,
      type: "json",
      success: function(data) {
        this.updateIssues(data);
      }.bind(this),
      error: function(xhr) {
        this.setState({error: xhr.statusText});
      }.bind(this)
    });
  },
  updateIssues: function(data) {
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

    var selectedSeverities = allSeverities;
    if (allSeverities.length > 1) {
      selectedSeverities = selectedSeverities.filter(function(i) { return i != "LOW" });
    }

    var selectedConfidences = allConfidences;
    if (allConfidences.length > 1) {
      selectedConfidences = selectedConfidences.filter(function(i) { return i != "LOW" });
    }

    this.updateIssueTypes(data.results.issues, selectedSeverities, selectedConfidences);

    this.setState({
      data: data,
      severity: selectedSeverities,
      allSeverities: allSeverities,
      confidence: selectedConfidences,
      allConfidences: allConfidences,
      issueType: null
    });
  },
  updateIssueTypes: function(issues, severities, confidences) {
    var allTypes = issues
      .filter(function(issue) {
        return severities.includes(issue.severity);
      })
      .filter(function(issue) {
        return confidences.includes(issue.confidence);
      })
      .map(function(issue) {
        return cleanupIssueType(issue.details).split(".")[0];
      })
      .sort()
      .filter(function(item, pos, ary) {
        return !pos || item != ary[pos - 1];
      });

    if (this.state.issueType && !allTypes.includes(this.state.issueType)) {
      this.setState({issueType: null});
    }

    this.setState({allIssueTypes: allTypes});
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

    if (this.state.data === undefined) {
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
              issueType={ this.state.issueType }
              allSeverities={ this.state.allSeverities } 
              allConfidences={ this.state.allConfidences }
              allIssueTypes={ this.state.allIssueTypes }
              onSeverity={ this.handleSeverity } 
              onConfidence={ this.handleConfidence } 
              onIssueType={ this.handleIssueType }
            />
          </div>
          <div className="column is-three-quarters">
            <Issues
              data={ this.state.data }
              severity={ this.state.severity }
              confidence={ this.state.confidence }
              issueType={ this.state.issueType }
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
      repo: e.target.value.trim(),
      valid: this.state.valid || re.test(e.target.value.trim())
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
    location.href = '/#' + this.state.repo;
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
  var repo = window.location.hash.replace(/^#\/?|\/$/g, "");

  if (!repo) {
    ReactDOM.render(
      <RepoSelector key={ Date.now() } />,
      document.getElementById("content")
    );
  } else {
    ReactDOM.render(
      <IssueBrowser key={ Date.now() } repo={ repo } />,
      document.getElementById("content")
    );
  }
}

window.addEventListener("hashchange", render, false);
window.dispatchEvent(new HashChangeEvent("hashchange"));
